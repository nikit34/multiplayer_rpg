package main

import (
	"time"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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

	box := tview.NewBox().SetBorder(true).SetTitle("multiplayer-rpg")
	box.SetDrawFunc(
		func(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
			width = width - 1
			height = height - 1
			centerY := y + height / 2
			centerX := x + width / 2

			for x := 1; x < width; x++ {
				for y := 1; y < height; y++ {
					screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
				}
			}
			screen.SetContent(centerX, centerY, 'O', nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
			for _, player := range game.Players {
				player.Mux.Lock()
				screen.SetContent(
					centerX + player.Position.X,
					centerY + player.Position.Y,
					player.Icon,
					nil,
					tcell.StyleDefault.Foreground(tcell.ColorRed),
				)
				player.Mux.Unlock()
			}
			return 0, 0, 0, 0
		},
	)

	box.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		currentPlayer.Mux.Lock()
		switch e.Key() {
		case tcell.KeyUp:
			currentPlayer.Direction = backend.DirectionUp
		case tcell.KeyDown:
			currentPlayer.Direction = backend.DirectionDown
		case tcell.KeyLeft:
			currentPlayer.Direction = backend.DirectionLeft
		case tcell.KeyRight:
			currentPlayer.Direction = backend.DirectionRight
		default:
			currentPlayer.Direction = backend.DirectionStop
		}
		currentPlayer.Mux.Unlock()
		return e
	})

	app := tview.NewApplication()

	go func() {
		for {
			app.Draw()
			time.Sleep(17 * time.Microsecond)
		}
	}()

	game.Start()

	if err := app.SetRoot(box, true).SetFocus(box).Run(); err != nil {
		panic(err)
	}
}