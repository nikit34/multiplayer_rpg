package client

import (
	"context"
	"io"
	"log"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"
	"google.golang.org/grpc"
)


type GameClient struct {
	CurrentPlayer *backend.Player
	Stream proto.Game_StreamClient
	Game *backend.Game
	View *frontend.View
}

func NewGameClient(conn grpc.ClientConnInterface, game *backend.Game, view *frontend.View) *GameClient {
	client := proto.NewGameClient(conn)
	stream, err := client.Stream(context.Background())
	if err != nil {
		log.Fatalf("open stream error %v", err)
	}

	view.OnDirectionChange = func(player *backend.Player) {
		direction := proto.Move_STOP
		switch player.Direction {
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
		stream.Send(&req)
	}

	return &GameClient{
		Stream: stream,
		Game: game,
		View: view,
	}
}

func (c *GameClient) Connect(playerName string) {
	c.CurrentPlayer = &backend.Player{
		Name: playerName,
		Direction: backend.DirectionStop,
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
			resp, err := c.Stream.Recv()
			if err == io.EOF {
				log.Fatalf("EOF")
				return
			}
			if err != nil {
				log.Fatalf("can not receive %v", err)
			}

			init := resp.GetInitialize()
			if init != nil {
				c.Game.Mux.Lock()
				c.CurrentPlayer.Position.X = init.Position.X
				c.CurrentPlayer.Position.Y = init.Position.Y
				c.Game.Players[c.CurrentPlayer.Name] = c.CurrentPlayer
				for _, player := range init.Players {
					c.Game.Players[player.Player] = &backend.Player{
						Position: backend.Coordinate{
							X: player.Position.X,
							Y: player.Position.Y,
						},
						Name:      player.Player,
						Direction: backend.DirectionStop,
						Icon:      'P',
					}
				}
				c.Game.Mux.Unlock()
				c.View.CurrentPlayer = c.CurrentPlayer
			}

			add := resp.GetAddplayer()
			if add != nil {
				newPlayer := backend.Player{
					Position: backend.Coordinate{
						X: add.Position.X,
						Y: add.Position.Y,
					},
					Name:      resp.Player,
					Direction: backend.DirectionStop,
					Icon:      'P',
				}
				c.Game.Mux.Lock()
				c.Game.Players[resp.Player] = &newPlayer
				c.Game.Mux.Unlock()
			}

			update := resp.GetUpdateplayer()
			if update != nil && c.Game.Players[resp.Player] != nil {
				c.Game.Players[resp.Player].Mux.Lock()
				c.Game.Players[resp.Player].Position.X = update.Position.X
				c.Game.Players[resp.Player].Position.Y = update.Position.Y
				c.Game.Players[resp.Player].Mux.Unlock()
			}
		}
	}()
}