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
		Direction: backend.DirectionStop,
	}

	game := backend.NewGame()
	game.Players = append(game.Players, &currentPlayer)
	game.CurrentPlayer = &currentPlayer

	app := frontend.NewView(&game)

	game.Start()
	app.Start()
}
