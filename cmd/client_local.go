package main

import (
	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
)

func main() {
	currentPlayer := backend.Player{
		Position: backend.Coordinate{X: -1, Y: -5},
		Name: "Alice",
		Icon: 'A',
	}

	game := backend.NewGame()
	game.Players[currentPlayer.Name] = &currentPlayer
	view := frontend.NewView(&game)
	view.CurrentPlayer = &currentPlayer

	game.Start()
	view.Start()
}
