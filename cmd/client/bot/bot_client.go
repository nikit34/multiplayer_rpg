package main

import (
	"flag"
	"log"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/bot"
	"github.com/nikit34/multiplayer_rpg_go/pkg/client"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	"github.com/nikit34/multiplayer_rpg_go/proto"
	"google.golang.org/grpc"
)


func main() {
	address := flag.String("address", ":8888", "Server address")
	flag.Parse()

	game := backend.NewGame()
	game.IsAuthoritative = false

	view := frontend.NewView(game)
	game.Start()

	conn, err := grpc.Dial(*address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can not connect with server %v", err)
	}

	grpcClient := proto.NewGameClient(conn)
	client := client.NewGameClient(game, view)

	bots := bot.NewBots(game)
	player := bots.AddBot("Bob")

	err = client.Connect(grpcClient, player.ID(), player.Name, "")
	if err != nil {
		log.Fatalf("connect request failed %v", err)
	}

	client.Start()
	view.Start()
	bots.Start()

	err = <-view.Done
	if err != nil {
		log.Fatal(err)
	}
}