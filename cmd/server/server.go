package main

import (
	"fmt"
	"flag"
	"log"
	"net"

	"github.com/nikit34/multiplayer_rpg_go/pkg/backend"
	"github.com/nikit34/multiplayer_rpg_go/pkg/server"
	proto "github.com/nikit34/multiplayer_rpg_go/proto"

	"google.golang.org/grpc"
)


func main() {
	port := flag.Int("port", 8888, "Port to listen on")
	password := flag.String("password", "", "Server password")
	flag.Parse()

	log.Printf("listening on port %d", *port)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	game := backend.NewGame()
	game.Start()

	s := grpc.NewServer()
	server := server.NewGameServer(game, *password)
	proto.RegisterGameServer(s, server)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
