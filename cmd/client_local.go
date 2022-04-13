package main

import (
	"log"
	"flag"
	"fmt"

	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/bot"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
)

func main() {
	numBots := flag.Int("bots", 1, "Number of bots to play against")
	flag.Parse()

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

	view := frontend.NewView(game)
	view.CurrentPlayer = currentPlayers[0].ID()
	bots := bot.NewBots(game)
	for i := 0; i < *numBots; i++ {
		bots.AddBot(fmt.Sprintf("Bob %d", i))
	}

	game.Start()
	view.Start()
	bots.Start()

	err := <-view.Done
	if err != nil {
		log.Fatal(err)
	}
}
