package main

import (
	"flag"
	"log"

	"github.com/nikit34/multiplayer_rpg/pkg/backend"
	"github.com/nikit34/multiplayer_rpg/pkg/bot"
	"github.com/nikit34/multiplayer_rpg/pkg/client"
	"github.com/nikit34/multiplayer_rpg/pkg/frontend"
	"github.com/nikit34/multiplayer_rpg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)


func main() {
	address := flag.String("address", ":8888", "Server address")
	flag.Parse()

	game := backend.NewGame()
	game.IsAuthoritative = false

	view := frontend.NewView(game)
	game.Start()

	conn, err := grpc.Dial(*address, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
