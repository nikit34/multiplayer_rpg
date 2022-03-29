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

	direction := proto.Laser_STOP

	switch change.Laser.Direction {
	case backend.DirectionUp:
		direction = proto.Laser_UP
	case backend.DirectionDown:
		direction = proto.Laser_DOWN
	case backend.DirectionLeft:
		direction = proto.Laser_LEFT
	case backend.DirectionRight:
		direction = proto.Laser_RIGHT
	default:
		return
	}

	position := change.Laser.GetPosition()

	resp := proto.Response{
		Action: &proto.Response_Addlaser{
			Addlaser: &proto.AddLaser{
				Starttime: timestamp,
				Position:  &proto.Coordinate{X: int32(position.X), Y: int32(position.Y)},
				Laser: &proto.Laser{
					Direction: direction,
					Uuid:      change.UUID.String(),
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
	s.Game.Mux.Lock()

	connect := req.GetConnect()
	currentPlayer := connect.GetPlayer()

	players := make([]*proto.Player, 0)
	for _, player := range s.Game.Players {
		players = append(players, &proto.Player{
			Player: player.Name,
			Position: &proto.Coordinate{
				X: int32(player.Position.X),
				Y: int32(player.Position.Y),
			},
		})
	}
	s.Game.Players[currentPlayer] = &backend.Player{
		Position:  backend.Coordinate{X: 10, Y: 10},
		Name:      currentPlayer,
		Icon:      'P',
	}
	s.Game.Mux.Unlock()

	resp := proto.Response{
		Action: &proto.Response_Initialize{
			Initialize: &proto.Initialize{
				Position: &proto.Coordinate{X: 10, Y: 10},
				Players:  players,
			},
		},
	}

	if err := srv.Send(&resp); err != nil {
		log.Printf("send error %v", err)
	}

	log.Printf("sent initialize message for %v", currentPlayer)

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

	direction := backend.DirectionStop

	switch move.Direction {
	case proto.Move_UP:
		direction = backend.DirectionUp
	case proto.Move_DOWN:
		direction = backend.DirectionDown
	case proto.Move_LEFT:
		direction = backend.DirectionLeft
	case proto.Move_RIGHT:
		direction = backend.DirectionRight
	}
	s.Game.ActionChannel <- backend.MoveAction{
		PlayerName: currentPlayer,
		Direction: direction,
	}
}

func (s *GameServer) HandleLaserRequest(currentPlayer string, req *proto.Request, srv proto.Game_StreamServer) {
	move := req.GetLaser()
	direction := backend.DirectionStop

	switch move.Direction {
	case proto.Laser_UP:
		direction = backend.DirectionUp
	case proto.Laser_DOWN:
		direction = backend.DirectionDown
	case proto.Laser_LEFT:
		direction = backend.DirectionLeft
	case proto.Laser_RIGHT:
		direction = backend.DirectionRight
	}
	s.Game.ActionChannel <- backend.LaserAction{
		PlayerName: currentPlayer,
		Direction:  direction,
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