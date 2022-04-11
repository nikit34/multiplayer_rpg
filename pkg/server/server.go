package server

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)

type client struct {
	StreamServer proto.Game_StreamServer
	Cancel context.CancelFunc
	ID uuid.UUID
}

type GameServer struct {
	proto.UnimplementedGameServer
	game    *backend.Game
	clients map[uuid.UUID]*client
	mu      sync.RWMutex
}

func (s *GameServer) broadcast(resp *proto.Response) {
	removals := []uuid.UUID{}

	s.mu.RLock()
	for id, currentClient := range s.clients {
		if err := currentClient.StreamServer.Send(resp); err != nil {
			log.Printf("%s - broadcast error %v", id, err)
			currentClient.Cancel()
			continue
		}
		log.Printf("%s - broadcasted %+v", resp, id)
	}
	s.mu.RUnlock()

	for _, id := range removals {
		s.removePlayer(id)
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
		log.Fatalf("unable to parse new round timestamp %v", s.game.NewRoundAt)
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

func (s *GameServer) removeClient(currentClient *client) {
	delete(s.clients, currentClient.ID)
	currentClient.Cancel()
}

func (s *GameServer) removePlayer(playerID uuid.UUID) {
	s.game.Mu.Lock()
	defer s.game.Mu.Unlock()

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

func (s *GameServer) handleConnectRequest(req *proto.Request, srv proto.Game_StreamServer) (uuid.UUID, error) {
	connect := req.GetConnect()
	icon, _ := utf8.DecodeRuneInString(strings.ToUpper(connect.Name))

	playerID, err := uuid.Parse(connect.Id)
	if err != nil {
		return playerID, err
	}

	re := regexp.MustCompile("^[a-zA-Z0-9]+$")
	if !re.MatchString(connect.Name) {
		return playerID, errors.New("invalid name provided")
	}

	spawnPoints := s.game.GetMapSpawnPoints()
	rand.Seed(time.Now().Unix())
	i := rand.Int() % len(spawnPoints)
	startCoordinate := spawnPoints[i]

	player := &backend.Player{
		Name:            connect.Name,
		Icon:            icon,
		IdentifierBase:  backend.IdentifierBase{UUID: playerID},
		CurrentPosition: startCoordinate,
	}

	s.game.Mu.Lock()
	s.game.AddEntity(player)
	s.game.Mu.Unlock()

	s.game.Mu.RLock()
	entities := make([]*proto.Entity, 0)
	for _, entity := range s.game.Entities {
		protoEntity := proto.GetProtoEntity(entity)
		if protoEntity != nil {
			entities = append(entities, protoEntity)
		}
	}
	s.game.Mu.RUnlock()

	resp := proto.Response{
		Action: &proto.Response_Initialize{
			Initialize: &proto.Initialize{
				Entities: entities,
			},
		},
	}

	if err := srv.Send(&resp); err != nil {
		s.removePlayer(playerID)
		return playerID, err
	}

	log.Printf("%s - sent initialize message", connect.Id)

	resp = proto.Response{
		Action: &proto.Response_AddEntity{
			AddEntity: &proto.AddEntity{
				Entity: proto.GetProtoEntity(player),
			},
		},
	}
	s.broadcast(&resp)

	return playerID, nil
}

func (s *GameServer) handleMoveRequest(req *proto.Request, currentClient *client) {
	move := req.GetMove()

	s.game.ActionChannel <- backend.MoveAction{
		ID:        currentClient.ID,
		Direction: proto.GetBackendDirection(move.Direction),
	}
}

func (s *GameServer) handleLaserRequest(req *proto.Request, currentClient *client) {
	laser := req.GetLaser()
	id, err := uuid.Parse(laser.Id)
	if err != nil {
		log.Printf(`%s - invalid laser ID "%s"`, currentClient.ID, laser.Id)
	}

	s.game.ActionChannel <- backend.LaserAction{
		OwnerID:   currentClient.ID,
		ID:        id,
		Direction: proto.GetBackendDirection(laser.Direction),
	}
}

const (
	clientTimeout = 15
	maxClients = 8
)

func (s *GameServer) Stream(srv proto.Game_StreamServer) error {
	if len(s.clients) >= maxClients {
		return errors.New("Server is full")
	}

	log.Println("start server")

	ctx, cancel := context.WithCancel(srv.Context())

	var currentClient *client

	lastMessage := time.Now()
	timeoutTicker := time.NewTicker(1 * time.Minute)

	go func() {
		for {
			if currentClient != nil && time.Now().Sub(lastMessage).Minutes() > clientTimeout {
				log.Printf("%s - user time out", currentClient.ID)
				cancel()
				return
			}
			<-timeoutTicker.C
		}
	} ()

	go func() {
		for {
			req, err := srv.Recv()
			if err != nil {
				log.Printf("receive error %v", err)
				cancel()
				return
			}
			log.Printf("got message %+v", req)


			if currentClient != nil {
				lastMessage = time.Now()
			}

			if currentClient == nil && req.GetConnect() != nil {
				playerID, err := s.handleConnectRequest(req, srv)
				if err != nil {
					log.Printf("%s - error when connecting %+v", playerID, err)
					cancel()
					return
				}

				s.mu.Lock()
				currentClient = &client{
					StreamServer: srv,
					Cancel:       cancel,
					ID:           playerID,
				}
				s.clients[playerID] = currentClient
				s.mu.Unlock()
			}

			if currentClient == nil {
				continue
			}


			switch req.GetAction().(type) {
			case *proto.Request_Move:
				s.handleMoveRequest(req, currentClient)
			case *proto.Request_Laser:
				s.handleLaserRequest(req, currentClient)
			}
		}
	}()

	<-ctx.Done()
	timeoutTicker.Stop()
	log.Printf(`stream done with error "%v"`, ctx.Err())

	if currentClient != nil {
		log.Printf("%s - removing client", currentClient.ID)
		s.mu.Lock()
		s.removeClient(currentClient)
		s.mu.Unlock()
		s.removePlayer(currentClient.ID)
	}
	return ctx.Err()
}
