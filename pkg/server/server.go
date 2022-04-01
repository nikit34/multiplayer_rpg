package server

import (
	"log"
	"sync"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)


type client struct {
	StreamServer proto.Game_StreamServer
}

type GameServer struct {
	proto.UnimplementedGameServer
	Game 		*backend.Game
	Clients 	map[uuid.UUID]*client
	Mux 		sync.RWMutex
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
				Position: proto.GetProtoCoordinate(change.Position),
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

	resp := proto.Response{
		Action: &proto.Response_Addlaser{
			Addlaser: &proto.AddLaser{
				Laser: &proto.Laser{
					Direction: proto.GetProtoDirection(change.Laser.Direction),
					Uuid:      change.UUID.String(),
					Starttime: timestamp,
					Position:  proto.GetProtoCoordinate(change.Laser.GetPosition()),
				},
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandleLaserRemoveChange(change backend.LaserRemoveChange) {
	resp := proto.Response{
		Action: &proto.Response_Removelaser{
			Removelaser: &proto.RemoveLaser{
				Uuid: change.UUID.String(),
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandlePlayerKilledChange(change backend.RemoveEntityChange) {
	resp := proto.Response{
		Action: &proto.Response_RemoveEntity{
			RemoveEntity: &proto.RemoveEntity{
				Entity: proto.GetProtoEntity(change.Entity),
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
			case backend.AddEntityChange:
				change := change.(backend.AddEntityChange)
				s.HandleAddEntityChange(change)
			case backend.RemoveEntityChange:
				change := change.(backend.RemoveEntityChange)
				s.HandleRemoveEntityChange(change)
			}
		}
	}()
}

func NewGameServer(game *backend.Game) *GameServer {
	server := &GameServer{
		Game: game,
		Clients: make(map[uuid.UUID]*client),
	}
	server.WatchChanges()
	return server
}

func (s *GameServer) RemoveClient(playerID uuid.UUID, srv proto.Game_StreamServer) {
	s.Mux.Lock()
	delete(s.Clients, playerID)
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

	playerID, err := uuid.Parse(connect.Id)
	if err != nil {

	}
	player := &backend.Player{
		Name: connect.Name,
		Icon: 'P',
		IdentifierBase: backend.IdentifierBase{UUID: playerID},
	}

	startCoordinate := backend.Coordinate{X: 0, Y: 0}
	player.Move(startCoordinate)
	s.Game.AddEntity(player)

	entities := make([]*proto.Entity, 0)
	for _, entity := range s.Game.Entities {
		protoEntity := proto.GetProtoEntity(entity)
		if protoEntity != nil {
			entities = append(entities, protoEntity)
		}
	}

	resp := proto.Response{
		Action: &proto.Response_Initialize{
			Initialize: &proto.Initialize{
				Entities: entities,
			},
		},
	}

	if err := srv.Send(&resp); err != nil {
		log.Printf("send error %v", err)
	}

	log.Printf("sent initialize message for %s", connect.Name)

	resp = proto.Response{
		Id: connect.Id,
		Action: &proto.Response_AddEntity{
			AddEntity: &proto.AddEntity{
				Entity: proto.GetProtoEntity(player),
			},
		},
	}
	s.Broadcast(&resp)

	s.Mux.Lock()
	s.Clients[player.ID()] = &client{
		StreamServer: srv,
	}
	s.Mux.Unlock()

	return player.ID()
}

func (s *GameServer) HandleMoveRequest(playerID uuid.UUID, req *proto.Request, srv proto.Game_StreamServer) {
	move := req.GetMove()

	s.Game.ActionChannel <- backend.MoveAction{
		ID: playerID,
		Direction:  proto.GetBackendDirection(move.Direction),
	}
}

func (s *GameServer) HandleLaserRequest(playerID uuid.UUID, req *proto.Request, srv proto.Game_StreamServer) {
	move := req.GetLaser()

	s.Game.ActionChannel <- backend.LaserAction{
		OwnerID: playerID,
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