package frontend

import (
	"time"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)


type View struct {
	Game *backend.Game
	App *tview.Application
	CurrentPlayer *backend.Player
}

func NewView(game *backend.Game) *View {
	app := tview.NewApplication()
	view := &View{
		Game: game,
		App: app,
	}

	box := tview.NewBox().SetBorder(true).SetTitle("multiplayer-rpg")
	box.SetDrawFunc(
		func(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
			view.Game.Mux.RLock()
			defer view.Game.Mux.RUnlock()

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
			for _, player := range view.Game.Players {
				player.Mux.RLock()
				screen.SetContent(
					centerX + int(player.Position.X),
					centerY + int(player.Position.Y),
					player.Icon,
					nil,
					tcell.StyleDefault.Foreground(tcell.ColorRed),
				)
				player.Mux.RUnlock()
			}
			for _, laser := range view.Game.Lasers {
				laserPosition := laser.GetPosition()
				screen.SetContent(
					centerX + laserPosition.X,
					centerY + laserPosition.Y,
					'x',
					nil,
					tcell.StyleDefault.Foreground(tcell.ColorYellow),
				)
			}
			return 0, 0, 0, 0
		},
	)

	box.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if view.CurrentPlayer == nil {
			return e
		}

		view.Game.Mux.RLock()
		defer view.Game.Mux.RUnlock()
		view.CurrentPlayer.Mux.RLock()
		defer view.CurrentPlayer.Mux.RUnlock()

		direction := backend.DirectionStop
		switch e.Key() {
		case tcell.KeyUp:
			direction = backend.DirectionUp
		case tcell.KeyDown:
			direction = backend.DirectionDown
		case tcell.KeyLeft:
			direction = backend.DirectionLeft
		case tcell.KeyRight:
			direction = backend.DirectionRight
		}
		if direction != backend.DirectionStop {
			view.Game.ActionChannel <- backend.MoveAction{
				PlayerName: view.CurrentPlayer.Name,
				Direction: direction,
			}
		}

		laserDirection := backend.DirectionStop
		switch e.Rune() {
		case 'w':
			laserDirection = backend.DirectionUp
		case 's':
			laserDirection = backend.DirectionDown
		case 'a':
			laserDirection = backend.DirectionLeft
		case 'd':
			laserDirection = backend.DirectionRight
		}
		if laserDirection != backend.DirectionStop {
			view.Game.ActionChannel <- backend.LaserAction{
				PlayerName: view.CurrentPlayer.Name,
				Direction: laserDirection,
			}
		}
		return e
	})

	app.SetRoot(box, true).SetFocus(box)
	return view
}

func (view *View) Start() error {
	go func() {
		for {
			view.App.Draw()
			time.Sleep(17 * time.Microsecond)
		}
	}()

	return view.App.Run()
}