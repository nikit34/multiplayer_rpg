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
	"google.golang.org/grpc/metadata"

	"github.com/nikit34/multiplayer_rpg/pkg/backend"
	proto "github.com/nikit34/multiplayer_rpg/proto"
)

type client struct {
	streamServer proto.Game_StreamServer
	lastMessage time.Time
	done chan error
	playerID uuid.UUID
	id uuid.UUID
}

type GameServer struct {
	proto.UnimplementedGameServer
	game    *backend.Game
	clients map[uuid.UUID]*client
	mu      sync.RWMutex
	password string
}

func (s *GameServer) broadcast(resp *proto.Response) {
	s.mu.Lock()
	for id, currentClient := range s.clients {
		if currentClient.streamServer == nil {
			continue
		}
		if err := currentClient.streamServer.Send(resp); err != nil {
			log.Printf("%s - broadcast error %v", id, err)
			currentClient.done <- errors.New("failed to broadcast message")
			continue
		}
		log.Printf("%s - broadcasted %+v", resp, id)
	}
	s.mu.Unlock()
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

func (s *GameServer) watchChanges() {
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

func (s *GameServer) watchTimeout() {
	timeoutTicker := time.NewTicker(1 * time.Minute)

	go func () {
		for {
			for _, client := range s.clients {
				if time.Since(client.lastMessage).Minutes() > clientTimeout {
					client.done <- errors.New("you have been timed out")
					return
				}
			}
			<- timeoutTicker.C
		}
	} ()
}

func NewGameServer(game *backend.Game, password string) *GameServer {
	server := &GameServer{
		game:    game,
		clients: make(map[uuid.UUID]*client),
		password: password,
	}
	server.watchChanges()
	server.watchTimeout()
	return server
}

func (s *GameServer) removeClient(id uuid.UUID) {
	s.mu.Lock()
	delete(s.clients, id)
	s.mu.Unlock()
}

func (s *GameServer) removePlayer(playerID uuid.UUID) {
	s.game.Mu.Lock()
	s.game.RemoveEntity(playerID)
	s.game.Mu.Unlock()

	resp := proto.Response{
		Action: &proto.Response_RemoveEntity{
			RemoveEntity: &proto.RemoveEntity{
				Id: playerID.String(),
			},
		},
	}
	s.broadcast(&resp)
}

func (s *GameServer) Connect(ctx context.Context, req *proto.ConnectRequest) (*proto.ConnectResponse, error) {
	if len(s.clients) >= maxClients {
		return nil, errors.New("The server is full")
	}

	playerID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, err
	}

	if req.Password != s.password {
		return nil, errors.New("invalid password provided")
	}

	s.game.Mu.RLock()
	if s.game.GetEntity(playerID) != nil {
		return nil, errors.New("duplicate player ID provided")
	}
	s.game.Mu.RUnlock()

	re := regexp.MustCompile("^[a-zA-Z0-9]+$")
	if !re.MatchString(req.Name) {
		return nil, errors.New("invalid name provided")
	}
	icon, _ := utf8.DecodeLastRuneInString(strings.ToUpper(req.Name))

	spawnPoints := s.game.GetMapByType()[backend.MapTypeSpawn]
	rand.Seed(time.Now().Unix())
	i := rand.Int() % len(spawnPoints)
	startCoordinate := spawnPoints[i]

	player := &backend.Player{
		Name: req.Name,
		Icon: icon,
		IdentifierBase: backend.IdentifierBase{UUID: playerID},
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
		Action: &proto.Response_AddEntity{
			AddEntity: &proto.AddEntity{
				Entity: proto.GetProtoEntity(player),
			},
		},
	}

	s.broadcast(&resp)

	s.mu.Lock()
	token := uuid.New()
	s.clients[token] = &client{
		id: token,
		playerID: playerID,
		done: make(chan error),
		lastMessage: time.Now(),
	}
	s.mu.Unlock()

	return &proto.ConnectResponse{
		Token:    token.String(),
		Entities: entities,
	}, nil
}

func (s *GameServer) handleMoveRequest(req *proto.Request, currentClient *client) {
	move := req.GetMove()

	s.game.ActionChannel <- backend.MoveAction{
		ID:        currentClient.playerID,
		Direction: proto.GetBackendDirection(move.Direction),
		Created: time.Now(),
	}
}

func (s *GameServer) handleLaserRequest(req *proto.Request, currentClient *client) {
	laser := req.GetLaser()
	id, err := uuid.Parse(laser.Id)
	if err != nil {
		currentClient.done <- errors.New("invalid laser ID provided")
		return
	}

	s.game.Mu.RLock()
	if s.game.GetEntity(id) != nil {
		currentClient.done <- errors.New("duplicate laser ID provided")
		return
	}
	s.game.Mu.RUnlock()

	s.game.ActionChannel <- backend.LaserAction{
		OwnerID:   currentClient.playerID,
		ID:        id,
		Direction: proto.GetBackendDirection(laser.Direction),
		Created: time.Now(),
	}
}

const (
	clientTimeout = 15
	maxClients = 8
)

func (s *GameServer) getClientFromContext(ctx context.Context) (*client, error) {
	headers, _ := metadata.FromIncomingContext(ctx)

	tokenRaw := headers["authorization"]
	if len(tokenRaw) == 0 {
		return nil, errors.New("no token provided")
	}

	token, err := uuid.Parse(tokenRaw[0])
	if err != nil {
		return nil, errors.New("cannot parse token")
	}

	s.mu.RLock()
	currentClient, ok := s.clients[token]
	s.mu.RUnlock()

	if !ok {
		return nil, errors.New("token not recognized")
	}
	return currentClient, nil
}

func (s *GameServer) Stream(srv proto.Game_StreamServer) error {
	ctx := srv.Context()
	currentClient, err := s.getClientFromContext(ctx)
	if err != nil {
		return err
	}
	if currentClient.streamServer != nil {
		return errors.New("stream already active")
	}
	currentClient.streamServer = srv

	log.Println("start new server")

	go func() {
		for {
			req, err := srv.Recv()
			if err != nil {
				log.Printf("receive error %v", err)
				currentClient.done <- errors.New("failed to receive request")
				return
			}
			log.Printf("got message %+v", req)
			currentClient.lastMessage = time.Now()

			switch req.GetAction().(type) {
			case *proto.Request_Move:
				s.handleMoveRequest(req, currentClient)
			case *proto.Request_Laser:
				s.handleLaserRequest(req, currentClient)
			}
		}
	}()

	var doneError error
	select {
	case <-ctx.Done():
		doneError = ctx.Err()

	case doneError = <-currentClient.done:
	}

	log.Printf(`stream done with error "%v"`, doneError)
	log.Printf("%s - removing client", currentClient.id)

	s.removeClient(currentClient.id)
	s.removePlayer(currentClient.playerID)

	return doneError
}
