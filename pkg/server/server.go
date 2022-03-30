package server

import (
	"log"
	"sync"

	"github.com/golang/protobuf/ptypes"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)


type client struct {
	StreamServer proto.Game_StreamServer
}

type GameServer struct {
	proto.UnimplementedGameServer
	Game *backend.Game
	Clients map[string]*client
	Mux sync.RWMutex
}

func (s *GameServer) Broadcast(resp *proto.Response) {
	s.Mux.RLock()
	for name, client := range s.Clients {
		if err := client.StreamServer.Send(resp); err != nil {
			log.Printf("broadcast error %v", err)
		}
		log.Printf("broadcasted %+v message to %s", resp, name)
	}
	s.Mux.RUnlock()
}

func (s *GameServer) HandlePositionChange(change backend.PositionChange) {
	resp := proto.Response{
		Player: change.PlayerName,
		Action: &proto.Response_Updateplayer{
			Updateplayer: &proto.UpdatePlayer{
				Position: &proto.Coordinate{
					X: int32(change.Position.X),
					Y: int32(change.Position.Y),
				},
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandleLaserChange(change backend.LaserChange) {
	timestamp, err := ptypes.TimestampProto(change.Laser.StartTime)
	if err != nil {
		return
	}

	position := change.Laser.GetPosition()

	resp := proto.Response{
		Action: &proto.Response_Addlaser{
			Addlaser: &proto.AddLaser{
				Laser: &proto.Laser{
					Direction: proto.GetProtoDirection(change.Laser.Direction),
					Uuid:      change.UUID.String(),
					Starttime: timestamp,
					Position:  &proto.Coordinate{X: int32(position.X), Y: int32(position.Y)},
				},
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) WatchChanges() {
	go func() {
		for {
			change := <-s.Game.ChangeChannel
			switch change.(type) {
			case backend.PositionChange:
				change := change.(backend.PositionChange)
				s.HandlePositionChange(change)
			case backend.LaserChange:
				change := change.(backend.LaserChange)
				s.HandleLaserChange(change)
			}
		}
	}()
}

func NewGameServer(game *backend.Game) *GameServer {
	server := &GameServer{
		Game: game,
		Clients: make(map[string]*client),
	}
	server.WatchChanges()
	return server
}

func (s *GameServer) RemoveClient(playerName string, srv proto.Game_StreamServer) {
	s.Mux.Lock()
	delete(s.Clients, playerName)
	s.Mux.Unlock()

	s.Game.Mux.Lock()
	delete(s.Game.Players, playerName)
	delete(s.Game.LastAction, playerName)
	s.Game.Mux.Unlock()

	resp := proto.Response{
		Player: playerName,
		Action: &proto.Response_Removeplayer{
			Removeplayer: &proto.RemovePlayer{},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandleConnectRequest(req *proto.Request, srv proto.Game_StreamServer) string {
	connect := req.GetConnect()
	currentPlayer := connect.GetPlayer()
	players := make([]*proto.Player, 0)

	s.Game.Mux.RLock()
	for _, player := range s.Game.Players {
		players = append(players, &proto.Player{
			Player: player.Name,
			Position: &proto.Coordinate{
				X: int32(player.Position.X),
				Y: int32(player.Position.Y),
			},
		})
	}

	lasers := make([]*proto.Laser, 0)
	for uuid, laser := range s.Game.Lasers {
		starttime, err := ptypes.TimestampProto(laser.StartTime)
		if err != nil {
			continue
		}

		lasers = append(lasers, &proto.Laser{
			Direction: proto.GetProtoDirection(laser.Direction),
			Uuid: uuid.String(),
			Starttime: starttime,
			Position: &proto.Coordinate{
				X: int32(laser.InitialPosition.X),
				Y: int32(laser.InitialPosition.Y),
			},
		})
	}
	s.Game.Mux.RUnlock()

	resp := proto.Response{
		Action: &proto.Response_Initialize{
			Initialize: &proto.Initialize{
				Position: &proto.Coordinate{X: 10, Y: 10},
				Players:  players,
				Lasers: lasers,
			},
		},
	}

	if err := srv.Send(&resp); err != nil {
		log.Printf("send error %v", err)
	}

	log.Printf("sent initialize message for %v", currentPlayer)

	s.Game.Mux.Lock()
	s.Game.Players[currentPlayer] = &backend.Player{
		Position: backend.Coordinate{X: 10, Y: 10},
		Name: currentPlayer,
		Icon: 'P',
	}
	s.Game.Mux.Unlock()

	resp = proto.Response{
		Player: currentPlayer,
		Action: &proto.Response_Addplayer{
			Addplayer: &proto.AddPlayer{
				Position: &proto.Coordinate{X: 10, Y: 10},
			},
		},
	}

	s.Broadcast(&resp)

	s.Mux.Lock()
	s.Clients[currentPlayer] = &client{
		StreamServer: srv,
	}
	s.Mux.Unlock()

	return currentPlayer
}

func (s *GameServer) HandleMoveRequest(currentPlayer string, req *proto.Request, srv proto.Game_StreamServer) {
	move := req.GetMove()

	s.Game.ActionChannel <- backend.MoveAction{
		PlayerName: currentPlayer,
		Direction:  proto.GetBackendDirection(move.Direction),
	}
}

func (s *GameServer) HandleLaserRequest(currentPlayer string, req *proto.Request, srv proto.Game_StreamServer) {
	move := req.GetLaser()

	s.Game.ActionChannel <- backend.LaserAction{
		PlayerName: currentPlayer,
		Direction:  proto.GetBackendDirection(move.Direction),
	}
}

func (s *GameServer) Stream(srv proto.Game_StreamServer) error {
	log.Println("start server")
	ctx := srv.Context()
	currentPlayer := ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := srv.Recv()
		if err != nil {
			log.Printf("receive error %v", err)
			if currentPlayer != "" {
				s.RemoveClient(currentPlayer, srv)
			}
			continue
		}

		if req.GetConnect() != nil {
			currentPlayer = s.HandleConnectRequest(req, srv)
		}

		if currentPlayer == "" {
			continue
		}

		switch req.GetAction().(type) {
		case *proto.Request_Move:
			s.HandleMoveRequest(currentPlayer, req, srv)
		case *proto.Request_Laser:
			s.HandleLaserRequest(currentPlayer, req, srv)
		}
	}
}