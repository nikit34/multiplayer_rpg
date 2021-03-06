package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	termutil "github.com/andrew-d/go-termutil"
	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg/pkg/backend"
	"github.com/nikit34/multiplayer_rpg/pkg/bot"
	"github.com/nikit34/multiplayer_rpg/pkg/frontend"
)

func main() {
	if !termutil.Isatty(os.Stdin.Fd()) {
		panic("this program must be run in a terminal")
	}

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
