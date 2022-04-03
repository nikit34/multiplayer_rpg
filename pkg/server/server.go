package server

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)

type client struct {
	StreamServer proto.Game_StreamServer
}

type GameServer struct {
	proto.UnimplementedGameServer
	Game    *backend.Game
	Clients map[uuid.UUID]*client
	Mux     sync.RWMutex
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

func (s *GameServer) HandleMoveChange(change backend.MoveChange) {
	resp := proto.Response{
		Action: &proto.Response_UpdateEntity{
			UpdateEntity: &proto.UpdateEntity{
				Entity: proto.GetProtoEntity(change.Entity),
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandleAddEntityChange(change backend.AddEntityChange) {
	resp := proto.Response{
		Action: &proto.Response_AddEntity{
			AddEntity: &proto.AddEntity{
				Entity: proto.GetProtoEntity(change.Entity),
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandleRemoveEntityChange(change backend.RemoveEntityChange) {
	resp := proto.Response{
		Action: &proto.Response_RemoveEntity{
			RemoveEntity: &proto.RemoveEntity{
				Id: change.Entity.ID().String(),
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandlePlayerRespawnChange(change backend.PlayerRespawnChange) {
	resp := proto.Response{
		Action: &proto.Response_PlayerRespawn{
			PlayerRespawn: &proto.PlayerRespawn{
				Player: proto.GetProtoPlayer(change.Player),
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) WatchChanges() {
	go func() {
		for {
			change := <-s.Game.ChangeChannel
			switch change_type := change.(type) {
			case backend.MoveChange:
				s.HandleMoveChange(change_type)
			case backend.AddEntityChange:
				s.HandleAddEntityChange(change_type)
			case backend.RemoveEntityChange:
				s.HandleRemoveEntityChange(change_type)
			case backend.PlayerRespawnChange:
				s.HandlePlayerRespawnChange(change_type)
			}
		}
	}()
}

func NewGameServer(game *backend.Game) *GameServer {
	server := &GameServer{
		Game:    game,
		Clients: make(map[uuid.UUID]*client),
	}
	server.WatchChanges()
	return server
}

func (s *GameServer) RemoveClient(playerID uuid.UUID, srv proto.Game_StreamServer) {
	s.Mux.Lock()
	delete(s.Clients, playerID)
	s.Mux.Unlock()

	s.Game.RemoveEntity(playerID)

	resp := proto.Response{
		Action: &proto.Response_RemoveEntity{
			RemoveEntity: &proto.RemoveEntity{
				Id: playerID.String(),
			},
		},
	}
	s.Broadcast(&resp)
}

func (s *GameServer) HandleConnectRequest(req *proto.Request, srv proto.Game_StreamServer) uuid.UUID {
	connect := req.GetConnect()

	playerID, err := uuid.Parse(connect.Id)
	if err != nil {

	}

	startCoordinate := backend.Coordinate{X: 0, Y: 0}
	
	player := &backend.Player{
		Name:           connect.Name,
		Icon:           'P',
		IdentifierBase: backend.IdentifierBase{UUID: playerID},
		CurrentPosition: startCoordinate,
	}

	player.Move(startCoordinate)
	s.Game.AddEntity(player)

	entities := make([]*proto.Entity, 0)
	s.Game.Mu.RLock()
	for _, entity := range s.Game.Entities {
		protoEntity := proto.GetProtoEntity(entity)
		if protoEntity != nil {
			entities = append(entities, protoEntity)
		}
	}
	s.Game.Mu.RUnlock()

	time.Sleep(time.Second * 1)

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
		ID:        playerID,
		Direction: proto.GetBackendDirection(move.Direction),
	}
}

func (s *GameServer) HandleLaserRequest(playerID uuid.UUID, req *proto.Request, srv proto.Game_StreamServer) {
	laser := req.GetLaser()
	id, err := uuid.Parse(laser.Id)
	if err != nil {
		return
	}

	s.Game.ActionChannel <- backend.LaserAction{
		OwnerID:   playerID,
		ID: id,
		Direction: proto.GetBackendDirection(laser.Direction),
	}
}

func (s *GameServer) Stream(srv proto.Game_StreamServer) error {
	log.Println("start server")
	ctx := srv.Context()
	var playerID uuid.UUID

	isConnected := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := srv.Recv()
		if err != nil {
			log.Printf("receive error %v", err)
			if isConnected {
				s.RemoveClient(playerID, srv)
			}
			continue
		}

		if req.GetConnect() != nil {
			playerID = s.HandleConnectRequest(req, srv)
			isConnected = true
		}

		if !isConnected {
			continue
		}
		log.Printf("got message %+v", req)

		switch req.GetAction().(type) {
		case *proto.Request_Move:
			s.HandleMoveRequest(playerID, req, srv)
		case *proto.Request_Laser:
			s.HandleLaserRequest(playerID, req, srv)
		}
	}
}
