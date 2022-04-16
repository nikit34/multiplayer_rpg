package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/nikit34/multiplayer_rpg/pkg/backend"
	"github.com/nikit34/multiplayer_rpg/pkg/bot"
	"github.com/nikit34/multiplayer_rpg/pkg/server"
	proto "github.com/nikit34/multiplayer_rpg/proto"

	"google.golang.org/grpc"
)


func main() {
	port := flag.Int("port", 8888, "Port to listen on")
	password := flag.String("password", "", "Server password")
	numBots := flag.Int("bots", 0, "Number of bots to add to server")
	flag.Parse()

	log.Printf("listening on port %d", *port)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	game := backend.NewGame()

	bots := bot.NewBots(game)
	for i := 0; i < *numBots; i++ {
		bots.AddBot(fmt.Sprintf("Bob %d", i))
	}

	game.Start()
	bots.Start()

	s := grpc.NewServer()
	server := server.NewGameServer(game, *password)
	proto.RegisterGameServer(s, server)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
