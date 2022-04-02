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
	Stream proto.Game_StreamClient
	Game *backend.Game
	View *frontend.View
}

func NewGameClient(stream proto.Game_StreamClient, game *backend.Game, view *frontend.View) *GameClient {
	return &GameClient{
		Stream: stream,
		Game: game,
		View: view,
	}
}

func (c *GameClient) Connect(playerID uuid.UUID, playerName string) {
	c.CurrentPlayer = playerID

	req := proto.Request{
		Action: &proto.Request_Connect{
			Connect: &proto.Connect{
				Id: playerID.String(),
				Name: playerName,
			},
		},
	}
	c.Stream.Send(&req)
}

func (c *GameClient) HandlePositionChange(change backend.PositionChange) {
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
	init := resp.GetInitialize()
	for _, entity := range init.Entities {
		c.Game.AddEntity(proto.GetBackendEntity(entity))
	}
	c.View.CurrentPlayer = c.CurrentPlayer
}

func (c *GameClient) HandleAddEntityChange(change backend.AddEntityChange) {
	switch change.Entity.(type) {
	case backend.Laser:
		laser := change.Entity.(backend.Laser)
		req := proto.Request{
			Action: &proto.Request_Laser{
				Laser: proto.GetProtoEntity(laser).GetLaser(),
			},
		}
		c.Stream.Send(&req)
	default:
		return
	}
}

func (c *GameClient) HandleAddEntityResponse(resp *proto.Response) {
	add := resp.GetAddEntity()
	entity := proto.GetBackendEntity(add.Entity)
	c.Game.AddEntity(entity)
}

func (c *GameClient) HandleUpdateEntityResponse(resp *proto.Response) {
	update := resp.GetUpdateEntity()
	entity := proto.GetBackendEntity(update.Entity)
	c.Game.UpdateEntity(entity)
}

func (c *GameClient) HandleRemoveEntityResponse(resp *proto.Response) {
	remove := resp.GetRemoveEntity()
	entity := proto.GetBackendEntity(remove.Entity)
	c.Game.RemoveEntity(entity.ID())
}

func (c *GameClient) Start() {
	go func() {
		for {
			change := <-c.Game.ChangeChannel

			switch change.(type) {
			case backend.PositionChange:
				change := change.(backend.PositionChange)
				c.HandlePositionChange(change)
			case backend.AddEntityChange:
				change := change.(backend.AddEntityChange)
				c.HandleAddEntityChange(change)
			}
		}
	}()

	go func() {
		for {
			resp, err := c.Stream.Recv()
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
			}
		}
	}()
}
