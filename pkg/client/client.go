package client

import (
	"io"
	"log"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
)


type GameClient struct {
	CurrentPlayer *backend.Player
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

func (c *GameClient) Connect(playerName string) {
	c.CurrentPlayer = &backend.Player{
		Name: playerName,
		Icon: 'P',
	}

	req := proto.Request{
		Action: &proto.Request_Connect{
			Connect: &proto.Connect{
				Player: playerName,
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

func (c *GameClient) HandleLaserChange(change backend.LaserChange) {
	req := proto.Request{
		Action: &proto.Request_Laser{
			Laser: &proto.Laser{
				Direction: proto.GetProtoDirection(change.Laser.Direction),
				Uuid:      change.UUID.String(),
			},
		},
	}
	c.Stream.Send(&req)
}

func (c *GameClient) HandleInitializeResponse(resp *proto.Response) {
	c.Game.Mux.Lock()
	defer c.Game.Mux.Unlock()

	init := resp.GetInitialize()
	c.CurrentPlayer.Position = proto.GetBackendCoordinate(init.Position)

	c.Game.Players[c.CurrentPlayer.Name] = c.CurrentPlayer
	for _, player := range init.Players {
		c.Game.Players[player.Player] = &backend.Player{
			Position:  proto.GetBackendCoordinate(player.Position),
			Name:      player.Player,
			Icon:      'P',
		}
	}

	for _, laser := range init.Lasers {
		laserUUID, err := uuid.Parse(laser.Uuid)
		if err != nil {
			continue
		}

		starttime, err := ptypes.Timestamp(laser.Starttime)
		if err != nil {
			continue
		}

		c.Game.Lasers[laserUUID] = backend.Laser{
			InitialPosition: proto.GetBackendCoordinate(laser.Position),
			Direction: proto.GetBackendDirection(laser.Direction),
			StartTime: starttime,
		}
	}
	c.View.CurrentPlayer = c.CurrentPlayer
}

func (c *GameClient) HandleAddPlayerResponse(resp *proto.Response) {
	add := resp.GetAddplayer()
	newPlayer := backend.Player{
		Position:  proto.GetBackendCoordinate(add.Position),
		Name:      resp.Player,
		Icon:      'P',
	}

	c.Game.Mux.Lock()
	c.Game.Players[resp.Player] = &newPlayer
	c.Game.Mux.Unlock()
}

func (c *GameClient) HandleUpdatePlayerResponse(resp *proto.Response) {
	c.Game.Mux.Lock()
	defer c.Game.Mux.Unlock()

	update := resp.GetUpdateplayer()
	if c.Game.Players[resp.Player] == nil {
		return
	}
	if resp.Player == c.CurrentPlayer.Name {
		return
	}

	c.Game.Players[resp.Player].Mux.Lock()
	c.Game.Players[resp.Player].Position = proto.GetBackendCoordinate(update.Position)
	c.Game.Players[resp.Player].Mux.Unlock()
}

func (c *GameClient) HandleRemovePlayerResponse(resp *proto.Response) {
	c.Game.Mux.Lock()
	defer c.Game.Mux.Unlock()
	delete(c.Game.Players, resp.Player)
	delete(c.Game.LastAction, resp.Player)
}

func (c *GameClient) HandleAddLaserResponse(resp *proto.Response) {
	addLaser := resp.GetAddlaser()
	protoLaser := addLaser.GetLaser()
	uuid, err := uuid.Parse(protoLaser.Uuid)
	if err != nil {
		return
	}

	startTime, err := ptypes.Timestamp(protoLaser.Starttime)
	if err != nil {
		return
	}

	c.Game.Mux.Lock()
	c.Game.Lasers[uuid] = backend.Laser{
		InitialPosition: proto.GetBackendCoordinate(protoLaser.Position),
		Direction:       proto.GetBackendDirection(protoLaser.Direction),
		StartTime:       startTime,
	}

	c.Game.Mux.Unlock()
}

func (c *GameClient) HandleRemoveLaserResponse(resp *proto.Response) {
	removeLaser := resp.GetRemovelaser()
	uuid, err := uuid.Parse(removeLaser.Uuid)
	if err != nil {
		return
	}

	c.Game.Mux.Lock()
	delete(c.Game.Lasers, uuid)
	c.Game.Mux.Unlock()
}

func (c *GameClient) HandlePlayerKilledResponse(resp *proto.Response) {
	playerKilled := resp.GetPlayerkilled()
	playerName := resp.GetPlayer()

	if c.Game.Players[playerName] == nil {
		return
	}

	c.Game.Players[playerName].Mux.Lock()
	c.Game.Players[playerName].Position = proto.GetBackendCoordinate(playerKilled.SpawnPosition)
	c.Game.Players[playerName].Mux.Unlock()
}

func (c *GameClient) Start() {
	go func() {
		for {
			change := <-c.Game.ChangeChannel

			switch change.(type) {
			case backend.PositionChange:
				change := change.(backend.PositionChange)
				c.HandlePositionChange(change)
			case backend.LaserChange:
				change := change.(backend.LaserChange)
				c.HandleLaserChange(change)
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
			case *proto.Response_Addplayer:
				c.HandleAddPlayerResponse(resp)
			case *proto.Response_Updateplayer:
				c.HandleUpdatePlayerResponse(resp)
			case *proto.Response_Removeplayer:
				c.HandleRemovePlayerResponse(resp)
			case *proto.Response_Addlaser:
				c.HandleAddLaserResponse(resp)
			case *proto.Response_Removelaser:
				c.HandleRemoveLaserResponse(resp)
			case *proto.Response_Playerkilled:
				c.HandlePlayerKilledResponse(resp)
			}
		}
	}()
}

func (c *GameClient) WatchChanges() {
	go func() {
		for {
			change := <- c.Game.ChangeChannel

			switch change.(type) {
			case backend.PositionChange:
				change := change.(backend.PositionChange)
				c.HandlePositionChange(change)
			case backend.LaserChange:
				change := change.(backend.LaserChange)
				c.HandleLaserChange(change)
			}
		}
	}()
}
