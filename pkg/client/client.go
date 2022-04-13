package client

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)


const (
	positionHistoryLimit = 5
)

type GameClient struct {
	CurrentPlayer uuid.UUID
	Stream        proto.Game_StreamClient
	Game          *backend.Game
	View          *frontend.View
	positionHistory []backend.Coordinate
}

func NewGameClient(game *backend.Game, view *frontend.View) *GameClient {
	return &GameClient{
		Game:   game,
		View:   view,
		positionHistory: make([]backend.Coordinate, positionHistoryLimit),
	}
}

func (c *GameClient) Connect(grpcClient proto.GameClient, playerID uuid.UUID, playerName string, password string) error {
	req := proto.ConnectRequest{
		Id:   playerID.String(),
		Name: playerName,
		Password: password,
	}

	resp, err := grpcClient.Connect(context.Background(), &req)
	if err != nil {
		return err
	}

	for _, entity := range resp.Entities {
		backendEntity := proto.GetBackendEntity(entity)
		if backendEntity == nil {
			return fmt.Errorf("can not get backend entity from %+v", entity)
		}

		c.Game.AddEntity(backendEntity)
	}

	header := metadata.New(map[string]string{"authorization": resp.Token})
	ctx := metadata.NewOutgoingContext(context.Background(), header)
	stream, err := grpcClient.Stream(ctx)
	if err != nil {
		return err
	}

	c.CurrentPlayer = playerID
	c.View.CurrentPlayer = playerID
	c.Stream = stream

	return nil
}

func (c *GameClient) handleMoveChange(change backend.MoveChange) {
	req := proto.Request{
		Action: &proto.Request_Move{
			Move: &proto.Move{
				Direction: proto.GetProtoDirection(change.Direction),
			},
		},
	}
	c.Stream.Send(&req)
	c.positionHistory = append([]backend.Coordinate{change.Position}, c.positionHistory[:positionHistoryLimit]...)
}

func (c *GameClient) handleAddEntityChange(change backend.AddEntityChange) {
	switch laser := change.Entity.(type) {
	case *backend.Laser:
		req := proto.Request{
			Action: &proto.Request_Laser{
				Laser: proto.GetProtoLaser(laser),
			},
		}
		c.Stream.Send(&req)
	}
}

func (c *GameClient) handleAddEntityResponse(resp *proto.Response) {
	add := resp.GetAddEntity()
	entity := proto.GetBackendEntity(add.Entity)
	if entity == nil {
		c.Exit(fmt.Sprintf("can not get backend entity from %+v", entity))
		return
	}

	laser, ok := entity.(*backend.Laser)
	if ok && laser.OwnerID == c.CurrentPlayer {
		return
	}
	c.Game.AddEntity(entity)
}

func (c *GameClient) handleUpdateEntityResponse(resp *proto.Response) {
	update := resp.GetUpdateEntity()
	entity := proto.GetBackendEntity(update.Entity)
	if entity == nil {
		c.Exit(fmt.Sprintf("can not get backend entity from %+v", entity))
		return
	}

	player, ok := entity.(*backend.Player)
	if ok && player.ID() == c.CurrentPlayer {
		for _, position := range c.positionHistory {
			if player.Position() == position {
				return
			}
		}
	}
	c.Game.UpdateEntity(entity)
}

func (c *GameClient) handleRemoveEntityResponse(resp *proto.Response) {
	remove := resp.GetRemoveEntity()
	id, err := uuid.Parse(remove.Id)
	if err != nil {
		c.Exit(fmt.Sprintf("error when parsing UUID: %v", err))
		return
	}
	c.Game.RemoveEntity(id)
}

func (c *GameClient) handlePlayerRespawnResponse(resp *proto.Response) {
	respawn := resp.GetPlayerRespawn()

	killedByID, err := uuid.Parse(respawn.KilledById)
	if err != nil {
		c.Exit(fmt.Sprintf("error when parsing UUID: %v", err))
		return
	}

	player := proto.GetBackendPlayer(respawn.Player)
	if player == nil {
		c.Exit(fmt.Sprintf("can not get backend player from %+v", respawn.Player))
		return
	}

	c.Game.AddScore(killedByID)
	c.Game.UpdateEntity(player)
}

func (c *GameClient) handleRoundOverResponse(resp *proto.Response) {
	respawn := resp.GetRoundOver()
	roundWinner, err := uuid.Parse(respawn.RoundWinnerId)
	if err != nil {
		c.Exit(fmt.Sprintf("error when parsing UUID: %v", err))
		return
	}

	c.Game.RoundWinner = roundWinner
	c.Game.NewRoundAt = respawn.NewRoundAt.AsTime()
	c.Game.WaitForRound = true
	c.Game.Score = make(map[uuid.UUID]int)
}

func (c *GameClient) handleRoundStartResponse(resp *proto.Response) {
	roundStart := resp.GetRoundStart()
	c.Game.WaitForRound = false

	for _, protoPlayer := range roundStart.Players {
		player := proto.GetBackendPlayer(protoPlayer)
		if player == nil {
			c.Exit(fmt.Sprintf("can not get backend player from %+v", protoPlayer))
			return
		}
		c.Game.AddEntity(player)
	}
}

func (c *GameClient) Exit(message string) {
	c.View.App.Stop()
	log.Println(message)
}

func (c *GameClient) Start() {
	go func() {
		for {
			change := <-c.Game.ChangeChannel
			switch type_change := change.(type) {
			case backend.MoveChange:
				c.handleMoveChange(type_change)
			case backend.AddEntityChange:
				c.handleAddEntityChange(type_change)
			}
		}
	}()

	go func() {
		for {
			resp, err := c.Stream.Recv()
			log.Printf("Recv %+v", resp)

			if err != nil {
				c.Exit(fmt.Sprintf("can not receive, error: %v", err))
				return
			}

			c.Game.Mu.Lock()
			switch resp.GetAction().(type) {
			case *proto.Response_AddEntity:
				c.handleAddEntityResponse(resp)
			case *proto.Response_UpdateEntity:
				c.handleUpdateEntityResponse(resp)
			case *proto.Response_RemoveEntity:
				c.handleRemoveEntityResponse(resp)
			case *proto.Response_PlayerRespawn:
				c.handlePlayerRespawnResponse(resp)
			case *proto.Response_RoundOver:
				c.handleRoundOverResponse(resp)
			case *proto.Response_RoundStart:
				c.handleRoundStartResponse(resp)
			}
			c.Game.Mu.Unlock()
		}
	}()
}
