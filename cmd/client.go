package main

import (
	"context"
	"io"
	"log"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/frontend"
	"github.com/nikit34/multiplayer_rpg_go/proto"

	"google.golang.org/grpc"
)

func main() {
	game := backend.NewGame()
	view := frontend.NewView(&game)

	game.Start()

	playerName := "Bob"

	conn, err := grpc.Dial(":8888", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("can not connect with server %v", err)
	}

	client := proto.NewGameClient(conn)
}