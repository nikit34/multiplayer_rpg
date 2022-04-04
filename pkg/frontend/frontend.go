package frontend

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type View struct {
	Game          *backend.Game
	App           *tview.Application
	CurrentPlayer uuid.UUID
	Paused        bool
	DrawCallbacks []func()
	ViewPort tview.Primitive
	Pages *tview.Pages
	RoundWait *tview.TextView
}

func setupViewPort(view *View) {
	box := tview.NewBox().SetBorder(true).
		SetTitle("multiplayer-rpg").
		SetBackgroundColor(tcell.ColorBlack)
	cameraX := 0
	cameraY := 0
	box.SetDrawFunc(
		func(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
			style := tcell.StyleDefault.Background(tcell.ColorBlack)

			view.Game.Mu.RLock()

			currentEntity := view.Game.GetEntity(view.CurrentPlayer)
			if currentEntity == nil {
				return 0, 0, 0, 0
			}
			currentPlayer := currentEntity.(*backend.Player)

			cameraDiffX := float64(cameraX - currentPlayer.Position().X)
			cameraDiffY := float64(cameraY - currentPlayer.Position().Y)
			if math.Abs(cameraDiffX) > 8 {
				if cameraDiffX <= 0 {
					cameraX++
				} else {
					cameraX--
				}
			}
			if math.Abs(cameraDiffY) > 8 {
				if cameraDiffY <= 0 {
					cameraY++
				} else {
					cameraY--
				}
			}

			view.Game.Mu.RUnlock()

			width = width - 1
			height = height - 1
			centerX := (x + width/2) - cameraX
			centerY := (y + height/2) - cameraY

			if centerX < width && centerX > 0 && centerY < height && centerY > 0 {
				screen.SetContent(centerX, centerY, 'C', nil, style.Foreground(tcell.ColorWhite))
			}
			view.Game.Mu.RLock()
			for _, entity := range view.Game.Entities {
				positioner, ok := entity.(backend.Positioner)
				if !ok {
					continue
				}

				position := positioner.Position()
				drawX := centerX + position.X
				drawY := centerY + position.Y
				if drawX >= width || drawX <= 0 || drawY >= height || drawY <= 0 {
					continue
				}

				var icon rune
				var color tcell.Color

				switch entity_type := entity.(type) {
				case *backend.Player:
					icon = entity_type.Icon
					color = tcell.ColorGreen
				case *backend.Laser:
					icon = 'x'
					color = tcell.ColorRed
				default:
					continue
				}

				screen.SetContent(
					drawX,
					drawY,
					icon,
					nil,
					style.Foreground(color),
				)
			}
			view.Game.Mu.RUnlock()
			return 0, 0, 0, 0
		},
	)

	box.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if view.Paused {
			return e
		}

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
				ID:        view.CurrentPlayer,
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
				OwnerID:   view.CurrentPlayer,
				ID: uuid.New(),
				Direction: laserDirection,
			}
		}
		return e
	})

	view.Pages.AddPage("viewport", box, true, true)
	view.ViewPort = box
}

func centeredModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, false).
			AddItem(nil, 0, 1, false), width, 1, false,
		).AddItem(nil, 0, 1, false)
}

func setupScoreModal(view *View) {
	textView := tview.NewTextView()
	textView.SetBorder(true).SetTitle("Score")
	modal := centeredModal(textView, 60, 23)

	callback := func() {
		text := ""
		view.Game.Mu.RLock()

		type PlayerScore struct {
			Name string
			Score int
		}
		playerScore := make([]PlayerScore, 0)

		for _, entity := range view.Game.Entities {
			player, ok := entity.(*backend.Player)
			if !ok {
				continue
			}

			score, ok := view.Game.Score[player.ID()]
			if !ok {
				score = 0
			}

			playerScore = append(playerScore, PlayerScore{
				Name: player.Name,
				Score: score,
			})
		}

		sort.Slice(playerScore, func(i, j int) bool {
			if playerScore[i].Score > playerScore[j].Score {
				return true
			}
			if strings.ToLower(playerScore[i].Name) < strings.ToLower(playerScore[j].Name) {
				return true
			}
			return false
		})

		for _, playerScore := range playerScore {
			text += fmt.Sprintf("%s - %d\n", playerScore.Name, playerScore.Score)
		}

		view.Game.Mu.RUnlock()
		textView.SetText(text)
	}

	view.DrawCallbacks = append(view.DrawCallbacks, callback)
	view.Pages.AddPage("score", modal, true, false)
}

func setupRoundWaitModal(view *View) {
	textView := tview.NewTextView()
	textView.SetTextAlign(tview.AlignCenter).
		SetScrollable(true).SetBorder(true).SetTitle("round complete")

	modal := centeredModal(textView, 60, 5)
	view.Pages.AddPage("roundwait", modal, true, false)

	callback := func() {
		if view.Game.WaitForRound {
			view.Pages.ShowPage("roundwait")

			seconds := int(view.Game.NewRoundAt.Sub(time.Now()).Seconds())
			if seconds < 0 {
				seconds = 0
			}

			player := view.Game.GetEntity(view.Game.RoundWinner).(*backend.Player)
			text := fmt.Sprintf("\nWinner: %s\n\n", player.Name)
			text += fmt.Sprintf("New round in %d seconds...", seconds)
			textView.SetText(text)
		} else {
			view.Pages.HidePage("roundwait")
			view.App.SetFocus(view.ViewPort)
		}
	}

	view.DrawCallbacks = append(view.DrawCallbacks, callback)
	view.Pages.AddPage("roundwait", modal, true, false)
}

func NewView(game *backend.Game) *View {
	app := tview.NewApplication()
	pages := tview.NewPages()
	view := &View{
		Game:   game,
		App:    app,
		Pages: pages,
		Paused: false,
		DrawCallbacks: make([]func(), 0),
	}

	setupViewPort(view)
	setupScoreModal(view)
	setupRoundWaitModal(view)

	app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Rune() == 'p' {
			pages.ShowPage("score")
		}
		if e.Key() == tcell.KeyESC {
			pages.HidePage("score")
			app.SetFocus(view.ViewPort)
		}
		return e
	})
	app.SetRoot(pages, true)
	return view
}

func (view *View) Start() error {
	go func() {
		for {
			for _, callback := range view.DrawCallbacks {
				view.App.QueueUpdate(callback)
			}
			view.App.Draw()
			time.Sleep(17 * time.Microsecond)
		}
	}()

	return view.App.Run()
}
