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

func (c *GameClient) HandleMoveChange(change backend.MoveChange) {
	req := proto.Request{
		Action: &proto.Request_Move{
			Move: &proto.Move{
				Direction: proto.GetProtoDirection(change.Direction),
			},
		},
	}
	c.Stream.Send(&req)
}

func (c *GameClient) HandleInitializeResponse(resp *proto.Response) {
	c.Game.Mu.Lock()
	defer c.Game.Mu.Unlock()

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

func (c *GameClient) HandleAddEntityChange(change backend.AddEntityChange) {
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

func (c *GameClient) HandleAddEntityResponse(resp *proto.Response) {
	c.Game.Mu.Lock()
	defer c.Game.Mu.Unlock()

	add := resp.GetAddEntity()
	entity := proto.GetBackendEntity(add.Entity)
	if entity == nil {
		return
	}
	c.Game.AddEntity(entity)
}

func (c *GameClient) HandleUpdateEntityResponse(resp *proto.Response) {
	c.Game.Mu.Lock()
	defer c.Game.Mu.Unlock()

	update := resp.GetUpdateEntity()
	entity := proto.GetBackendEntity(update.Entity)
	if entity == nil {
		return
	}
	c.Game.UpdateEntity(entity)
}

func (c *GameClient) HandleRemoveEntityResponse(resp *proto.Response) {
	c.Game.Mu.Lock()
	defer c.Game.Mu.Unlock()

	remove := resp.GetRemoveEntity()
	id, err := uuid.Parse(remove.Id)
	if err != nil {
		return
	}
	c.Game.RemoveEntity(id)
}

func (c *GameClient) HandlePlayerRespawnResponse(resp *proto.Response) {
	c.Game.Mu.Lock()
	defer c.Game.Mu.Unlock()

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

func (c *GameClient) Start() {
	go func() {
		for {
			change := <-c.Game.ChangeChannel
			switch type_change := change.(type) {
			case backend.MoveChange:
				c.HandleMoveChange(type_change)
			case backend.AddEntityChange:
				c.HandleAddEntityChange(type_change)
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

			switch resp.GetAction().(type) {
			case *proto.Response_Initialize:
				c.HandleInitializeResponse(resp)
			case *proto.Response_AddEntity:
				c.HandleAddEntityResponse(resp)
			case *proto.Response_UpdateEntity:
				c.HandleUpdateEntityResponse(resp)
			case *proto.Response_RemoveEntity:
				c.HandleRemoveEntityResponse(resp)
			case *proto.Response_PlayerRespawn:
				c.HandlePlayerRespawnResponse(resp)
			}
		}
	}()
}
