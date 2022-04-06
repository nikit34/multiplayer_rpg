package server

import (
	"log"
	"sync"
	"time"

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
	game    *backend.Game
	clients map[uuid.UUID]*client
	mu      sync.RWMutex
}

func (s *GameServer) broadcast(resp *proto.Response) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name, client := range s.clients {
		if err := client.StreamServer.Send(resp); err != nil {
			log.Printf("broadcast error %v", err)
		}
		log.Printf("broadcasted %+v message to %s", resp, name)
	}
}

func (s *GameServer) handleMoveChange(change backend.MoveChange) {
	resp := proto.Response{
		Action: &proto.Response_UpdateEntity{
			UpdateEntity: &proto.UpdateEntity{
				Entity: proto.GetProtoEntity(change.Entity),
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) handleAddEntityChange(change backend.AddEntityChange) {
	resp := proto.Response{
		Action: &proto.Response_AddEntity{
			AddEntity: &proto.AddEntity{
				Entity: proto.GetProtoEntity(change.Entity),
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) handleRemoveEntityChange(change backend.RemoveEntityChange) {
	resp := proto.Response{
		Action: &proto.Response_RemoveEntity{
			RemoveEntity: &proto.RemoveEntity{
				Id: change.Entity.ID().String(),
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) handlePlayerRespawnChange(change backend.PlayerRespawnChange) {
	resp := proto.Response{
		Action: &proto.Response_PlayerRespawn{
			PlayerRespawn: &proto.PlayerRespawn{
				Player:     proto.GetProtoPlayer(change.Player),
				KilledById: change.KilledByID.String(),
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) handleRoundOverChange(change backend.RoundOverChange) {
	s.game.Mu.RLock()
	defer s.game.Mu.RUnlock()
	timestamp, err := ptypes.TimestampProto(s.game.NewRoundAt)
	if err != nil {
		return
	}
	resp := proto.Response{
		Action: &proto.Response_RoundOver{
			RoundOver: &proto.RoundOver{
				RoundWinnerId: s.game.RoundWinner.String(),
				NewRoundAt:    timestamp,
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) handleRoundStartChange(change backend.RoundStartChange) {
	players := []*proto.Player{}
	s.game.Mu.RLock()
	for _, entity := range s.game.Entities {
		player, ok := entity.(*backend.Player)
		if !ok {
			continue
		}
		players = append(players, proto.GetProtoPlayer(player))
	}
	s.game.Mu.RUnlock()
	resp := proto.Response{
		Action: &proto.Response_RoundStart{
			RoundStart: &proto.RoundStart{
				Players: players,
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) WatchChanges() {
	go func() {
		for {
			change := <-s.game.ChangeChannel
			switch change_type := change.(type) {
			case backend.MoveChange:
				s.handleMoveChange(change_type)
			case backend.AddEntityChange:
				s.handleAddEntityChange(change_type)
			case backend.RemoveEntityChange:
				s.handleRemoveEntityChange(change_type)
			case backend.PlayerRespawnChange:
				s.handlePlayerRespawnChange(change_type)
			case backend.RoundOverChange:
				s.handleRoundOverChange(change_type)
			case backend.RoundStartChange:
				s.handleRoundStartChange(change_type)
			}
		}
	}()
}

func NewGameServer(game *backend.Game) *GameServer {
	server := &GameServer{
		game:    game,
		clients: make(map[uuid.UUID]*client),
	}
	server.WatchChanges()
	return server
}

func (s *GameServer) removeClient(playerID uuid.UUID, srv proto.Game_StreamServer) {
	s.game.Mu.Lock()
	defer s.game.Mu.Unlock()

	s.mu.Lock()
	delete(s.clients, playerID)
	s.mu.Unlock()

	s.game.RemoveEntity(playerID)

	resp := proto.Response{
		Action: &proto.Response_RemoveEntity{
			RemoveEntity: &proto.RemoveEntity{
				Id: playerID.String(),
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) handleConnectRequest(req *proto.Request, srv proto.Game_StreamServer) uuid.UUID {
	s.game.Mu.Lock()
	defer s.game.Mu.Unlock()

	connect := req.GetConnect()

	playerID, err := uuid.Parse(connect.Id)
	if err != nil {

	}

	startCoordinate := backend.Coordinate{X: 0, Y: 0}

	player := &backend.Player{
		Name:            connect.Name,
		Icon:            'P',
		IdentifierBase:  backend.IdentifierBase{UUID: playerID},
		CurrentPosition: startCoordinate,
	}

	player.Move(startCoordinate)
	s.game.AddEntity(player)

	entities := make([]*proto.Entity, 0)
	for _, entity := range s.game.Entities {
		protoEntity := proto.GetProtoEntity(entity)
		if protoEntity != nil {
			entities = append(entities, protoEntity)
		}
	}

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
	s.broadcast(&resp)

	s.mu.Lock()
	s.clients[player.ID()] = &client{
		StreamServer: srv,
	}
	s.mu.Unlock()

	return player.ID()
}

func (s *GameServer) handleMoveRequest(playerID uuid.UUID, req *proto.Request, srv proto.Game_StreamServer) {
	move := req.GetMove()

	s.game.ActionChannel <- backend.MoveAction{
		ID:        playerID,
		Direction: proto.GetBackendDirection(move.Direction),
	}
}

func (s *GameServer) handleLaserRequest(playerID uuid.UUID, req *proto.Request, srv proto.Game_StreamServer) {
	laser := req.GetLaser()
	id, err := uuid.Parse(laser.Id)
	if err != nil {
		return
	}

	s.game.ActionChannel <- backend.LaserAction{
		OwnerID:   playerID,
		ID:        id,
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
				s.removeClient(playerID, srv)
			}
			continue
		}

		if req.GetConnect() != nil {
			playerID = s.handleConnectRequest(req, srv)
			isConnected = true
		}

		if !isConnected {
			continue
		}
		log.Printf("got message %+v", req)

		switch req.GetAction().(type) {
		case *proto.Request_Move:
			s.handleMoveRequest(playerID, req, srv)
		case *proto.Request_Laser:
			s.handleLaserRequest(playerID, req, srv)
		}
	}
}
