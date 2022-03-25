package client

import (
	"io"
	"log"

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

func (c *GameClient) WatchChanges() {
	go func() {
		for {
			change := <- c.Game.ChangeChannel
			switch change.(type) {
			case backend.PositionChange:
				change := change.(backend.PositionChange)
				c.HandlePositionChange(change)
			}
		}
	}()
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

func (c *GameClient) Start() {
	go func() {
		for {
			change := <-c.Game.ChangeChannel
			switch change.(type) {
			case backend.PositionChange:
				change := change.(backend.PositionChange)
				c.HandlePositionChange(change)
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
			}
		}
	}()
}

func (c *GameClient) HandlePositionChange(change backend.PositionChange) {
	direction := proto.Move_STOP
	switch change.Direction {
	case backend.DirectionUp:
		direction = proto.Move_UP
	case backend.DirectionDown:
		direction = proto.Move_DOWN
	case backend.DirectionLeft:
		direction = proto.Move_LEFT
	case backend.DirectionRight:
		direction = proto.Move_RIGHT 
	}
	req := proto.Request{
		Action: &proto.Request_Move{
			Move: &proto.Move{
				Direction: direction,
			},
		},
	}
	c.Stream.Send(&req)
}

func (c *GameClient) HandleInitializeResponse(resp *proto.Response) {
	init := resp.GetInitialize()
	c.Game.Mux.Lock()
	c.CurrentPlayer.Position.X = int(init.Position.X)
	c.CurrentPlayer.Position.Y = int(init.Position.Y)
	c.Game.Players[c.CurrentPlayer.Name] = c.CurrentPlayer
	for _, player := range init.Players {
		c.Game.Players[player.Player] = &backend.Player{
			Position: backend.Coordinate{
				X: int(player.Position.X),
				Y: int(player.Position.Y),
			},
			Name:      player.Player,
			Icon:      'P',
		}
	}
	c.Game.Mux.Unlock()
	c.View.CurrentPlayer = c.CurrentPlayer
}

func (c *GameClient) HandleAddPlayerResponse(resp *proto.Response) {
	add := resp.GetAddplayer()
	newPlayer := backend.Player{
		Position: backend.Coordinate{
			X: int(add.Position.X),
			Y: int(add.Position.Y),
		},
		Name:      resp.Player,
		Icon:      'P',
	}
	c.Game.Mux.Lock()
	c.Game.Players[resp.Player] = &newPlayer
	c.Game.Mux.Unlock()
}

func (c *GameClient) HandleUpdatePlayerResponse(resp *proto.Response) {
	update := resp.GetUpdateplayer()
	if c.Game.Players[resp.Player] == nil {
		return
	}
	if resp.Player == c.CurrentPlayer.Name {
		return
	}
	c.Game.Players[resp.Player].Mux.Lock()
	c.Game.Players[resp.Player].Position.X = int(update.Position.X)
	c.Game.Players[resp.Player].Position.Y = int(update.Position.Y)
	c.Game.Players[resp.Player].Mux.Unlock()
}
