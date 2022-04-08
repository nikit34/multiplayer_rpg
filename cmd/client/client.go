package main

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/client"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	"github.com/nikit34/multiplayer_rpg_go/proto"

	"google.golang.org/grpc"
	"github.com/google/uuid"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)


type connectInfo struct {
	PlayerName string
	Address string
}

func connectApp(info *connectInfo) *tview.Application {
	backgroundColor := tcell.Color234
	app := tview.NewApplication()
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)
	flex.SetBorder(true).
		SetTitle("Connect to tshooter server").
		SetBackgroundColor(backgroundColor)
	errors := tview.NewTextView().
		SetText(" Use the tab key to change fields, and enter to submit")
	errors.SetBackgroundColor(backgroundColor)
	form := tview.NewForm()
	form.AddInputField("Player name", "", 16, nil, nil).
		AddInputField("Server address", ":8888", 32, nil, nil).
		AddButton("Connect", func() {
			info.PlayerName = form.GetFormItem(0).(*tview.InputField).GetText()
			info.Address = form.GetFormItem(1).(*tview.InputField).GetText()
			if info.PlayerName == "" || info.Address == "" {
				errors.SetText(" All fields are required.")
				return
			}
			app.Stop()
		}).
		AddButton("Quit", func() {
			app.Stop()
		})
	form.SetLabelColor(tcell.ColorWhite).
		SetButtonBackgroundColor(tcell.Color24).
		SetFieldBackgroundColor(tcell.Color24).
		SetBackgroundColor(backgroundColor)
	flex.AddItem(errors, 1, 1, false)
	flex.AddItem(form, 0, 1, false)
	app.SetRoot(flex, true).SetFocus(form)
	return app
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func main() {
	game := backend.NewGame()
	game.IsAuthoritative = false
	view := frontend.NewView(game)

	game.Start()

	info := connectInfo{}
	connectApp := connectApp(&info)
	connectApp.Run()

	conn, err := grpc.Dial(info.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can not connect with server %v", err)
	}

	grpcClient := proto.NewGameClient(conn)
	stream, err := grpcClient.Stream(context.Background())
	if err != nil {
		log.Fatalf("open stream error %v", err)
	}

	ctx := stream.Context()

	ctx, cancel := context.WithCancel(stream.Context())
	client := client.NewGameClient(stream, cancel, game, view)
	client.Start()

	playerID := uuid.New()
	client.Connect(playerID, info.PlayerName)

	view.Start()

	select {
	case <-ctx.Done():
		view.App.Stop()
		if err := ctx.Err(); err != nil {
			log.Println(err)
		}
	case <-view.Quit:
		return
	}
}
