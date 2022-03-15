package main

import (
	"io"
	"log"
	"net"

	"github.com/nikit34/multiplayer_rpg_go/proto"

	"google.golang.org/grpc"
)


type server struct {
	proto.UnimplementedGameServer
}

func (s server) Stream(srv proto.Game_streamServer) error {
	log.Println("start server")
	ctx := srv.Context()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := srv.Recv()
		if err == io.EOF {
			log.Println("exit")
			return nil
		}
		if err != nil {
			log.Printf("receive error %v", err)
			continue
		}

		// if move := req.GetMove() {

		// }

		max := req.Num
		resp := pb.Response{
			Result: max,
		}
		if err := srv.Send(&resp); err != nil {
			log.Printf("send error %v", err)
		}
		log.Printf("send new max=%d", max)
	}
}

func main() {
	lis, err := net.Listen("tcp", ":696969")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterGameServer(s, server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}