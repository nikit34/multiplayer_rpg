package main

import (
	"log"

	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/bot"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
)

func main() {
	currentPlayers := []backend.Player{{
			Name:            "Alice",
			Icon:            'A',
			IdentifierBase:  backend.IdentifierBase{UUID: uuid.New()},
			CurrentPosition: backend.Coordinate{X: -1, Y: -5},
		}, {
			Name:            "Bob",
			Icon:            'B',
			IdentifierBase:  backend.IdentifierBase{UUID: uuid.New()},
			CurrentPosition: backend.Coordinate{X: 0, Y: 0},
		},
	}

	game := backend.NewGame()

	game.AddEntity(&currentPlayers[0])
	game.AddEntity(&currentPlayers[1])
	bots := bot.NewBots(game)
	bots.AddBot("Dave")

	view := frontend.NewView(game)
	view.CurrentPlayer = currentPlayers[0].ID()

	game.Start()
	view.Start()
	bots.Start()

	err := <-view.Done
	if err != nil {
		log.Fatal(err)
	}
}
