package client

import (
	"io"
	"log"

	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)


type GameClient struct {
	CurrentPlayer uuid.UUID
	Stream        proto.Game_StreamClient
	Game          *backend.Game
	View          *frontend.View
}

func NewGameClient(stream proto.Game_StreamClient, game *backend.Game, view *frontend.View) *GameClient {
	return &GameClient{
		Stream: stream,
		Game:   game,
		View:   view,
	}
}

func (c *GameClient) Connect(playerID uuid.UUID, playerName string) {
	c.View.Paused = true
	c.CurrentPlayer = playerID

	req := proto.Request{
		Action: &proto.Request_Connect{
			Connect: &proto.Connect{
				Id:   playerID.String(),
				Name: playerName,
			},
		},
	}
	c.Stream.Send(&req)
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
}

func (c *GameClient) handleInitializeResponse(resp *proto.Response) {
	init := resp.GetInitialize()
	for _, entity := range init.Entities {
		backendEntity := proto.GetBackendEntity(entity)
		if backendEntity == nil {
			return
		}
		c.Game.AddEntity(backendEntity)
	}
	c.View.CurrentPlayer = c.CurrentPlayer
	c.View.Paused = false
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
	default:
		return
	}
}

func (c *GameClient) handleAddEntityResponse(resp *proto.Response) {
	add := resp.GetAddEntity()
	entity := proto.GetBackendEntity(add.Entity)
	if entity == nil {
		return
	}
	c.Game.AddEntity(entity)
}

func (c *GameClient) handleUpdateEntityResponse(resp *proto.Response) {
	update := resp.GetUpdateEntity()
	entity := proto.GetBackendEntity(update.Entity)
	if entity == nil {
		return
	}
	c.Game.UpdateEntity(entity)
}

func (c *GameClient) handleRemoveEntityResponse(resp *proto.Response) {
	remove := resp.GetRemoveEntity()
	id, err := uuid.Parse(remove.Id)
	if err != nil {
		return
	}
	c.Game.RemoveEntity(id)
}

func (c *GameClient) handlePlayerRespawnResponse(resp *proto.Response) {
	respawn := resp.GetPlayerRespawn()

	killedByID, err := uuid.Parse(respawn.KilledById)
	if err != nil {
		return
	}

	player := proto.GetBackendPlayer(respawn.Player)
	if player == nil {
		return
	}

	c.Game.AddScore(killedByID)
	c.Game.UpdateEntity(player)
}

func (c *GameClient) handleRoundOverResponse(resp *proto.Response) {
	respawn := resp.GetRoundOver()
	roundWinner, err := uuid.Parse(respawn.RoundWinnerId)
	if err != nil {
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
		c.Game.AddEntity(player)
	}
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
			if err == io.EOF {
				log.Fatalf("EOF")
				return
			}
			if err != nil {
				log.Fatalf("can not receive %v", err)
			}

			c.Game.Mu.Lock()
			switch resp.GetAction().(type) {
			case *proto.Response_Initialize:
				c.handleInitializeResponse(resp)
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
