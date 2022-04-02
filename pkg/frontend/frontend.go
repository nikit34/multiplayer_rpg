package frontend

import (
	"time"

	"github.com/google/uuid"
	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)


type View struct {
	Game *backend.Game
	App *tview.Application
	CurrentPlayer uuid.UUID
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
			view.Game.Mu.RLock()
			defer view.Game.Mu.RUnlock()

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
			for _, entity := range view.Game.Entities {
				positioner, ok := entity.(backend.Positioner)
				if !ok {
					continue
				}
				position := positioner.Position()

				var icon rune
				var color tcell.Color

				switch entity.(type) {
				case *backend.Player:
					icon = entity.(*backend.Player).Icon
					color = tcell.ColorGreen
				case *backend.Laser:
					icon = 'x'
					color = tcell.ColorRed
				default:
					continue
				}

				screen.SetContent(
					centerX + position.X,
					centerY + position.Y,
					icon,
					nil,
					tcell.StyleDefault.Foreground(color),
				)
			}
			return 0, 0, 0, 0
		},
	)

	box.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if view.CurrentPlayer.String() == "" {
			return e
		}

		view.Game.Mu.RLock()
		defer view.Game.Mu.RUnlock()

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
				ID:         view.CurrentPlayer,
				Direction:  direction,
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
				OwnerID:    view.CurrentPlayer,
				Direction:  laserDirection,
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