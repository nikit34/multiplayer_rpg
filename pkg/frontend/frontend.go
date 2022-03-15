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
}

func NewView(game *backend.Game) View {
	app := tview.NewApplication()
	view := View{
		Game: game,
		App: app,
	}

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
		view.Game.CurrentPlayer.Mux.Lock()
		switch e.Key() {
		case tcell.KeyUp:
			view.Game.CurrentPlayer.Direction = backend.DirectionUp
		case tcell.KeyDown:
			view.Game.CurrentPlayer.Direction = backend.DirectionDown
		case tcell.KeyLeft:
			view.Game.CurrentPlayer.Direction = backend.DirectionLeft
		case tcell.KeyRight:
			view.Game.CurrentPlayer.Direction = backend.DirectionRight
		}
		view.Game.CurrentPlayer.Mux.Unlock()
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