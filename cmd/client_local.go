package main

import (
	"github.com/google/uuid"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
)

func main() {
	currentPlayer := backend.Player{
		Name:           "Alice",
		Icon:           'A',
		IdentifierBase: backend.IdentifierBase{uuid.New()},
	}
	currentPlayer.Move(backend.Coordinate{X: -1, Y: -5})

	game := backend.NewGame()
	game.AddEntity(&currentPlayer)
	view := frontend.NewView(game)
	view.CurrentPlayer = &currentPlayer

	game.Start()
	view.Start()
}
