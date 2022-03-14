package main

import (
	"sync"
	"time"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)


type Coordinate struct {
	X int
	Y int
}

type Direction int32

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
	DirectionStop
)

type Player struct {
	Position Coordinate
	Direction Direction
	Name string
	Icon rune
	LastMove time.Time
	Mux sync.Mutex
}

type Game struct {
	Players []*Player
}

func main() {
	currentPlayer := Player{
		Position: Coordinate{X: -1, Y: -5},
		Name: "Alice",
		Icon: 'A',
		LastMove:  time.Time{},
		Direction: DirectionStop,
	}

	game := Game{Players: []*Player{
		&currentPlayer,
		&Player{
			Position: Coordinate{X: 10, Y: 10},
			Name: "Bella",
			Icon: 'B',
			LastMove: time.Time{},
			Direction: DirectionStop,
		},
	}}

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
					tcell.StyleDefault.Foreground(tcell.ColorRed)
				)
				player.Mux.Unlock()
			}
			return 0, 0, 0, 0
		}
	)

	box.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		currentPlayer.Mux.Lock()
		switch e.Key() {
		case tcell.KeyUp:
			currentPlayer.Direction = DirectionUp
		case tcell.KeyDown:
			currentPlayer.Direction = DirectionDown
		case tcell.KeyLeft:
			currentPlayer.Direction = DirectionLeft
		case tcell.KeyRight:
			currentPlayer.Direction = DirectionRight
		default:
			currentPlayer.Direction = DirectionStop
		}
		currentPlayer.Mux.Unlock()
		return e
	})

	app := tview.NewApplication()

	go func() {
		for {
			for _, player := range game.Players {
				player.Mux.Lock()
				if player.Direction == DirectionStop || player.LastMove.After(time.Now().Add(-50 * time.Millisecond)) {
					player.Direction = DirectionStop
					player.Mux.Unlock()
					continue
				}
				switch player.Direction {
				case DirectionUp:
					player.Position.Y -= 1
				case DirectionDown:
					player.Position.Y += 1
				case DirectionLeft:
					player.Position.X -= 1
				case DirectionRight:
					player.Position.X += 1
				}
				player.Direction = DirectionStop
				player.LastMove = time.Now()
				player.Mux.Unlock()
			}
		}
	}()

	if err := app.SetRoot(box, true).SetFocus(box).Run(); err != nil {
		panic(err)
	}
}