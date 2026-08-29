package main

import (
	"cli_tool/gen/event_service"
	"cli_tool/server"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	eventServer, err := server.NewEventServer(os.Getenv("RPC_ENDPOINT"), os.Getenv("NETWORK_PASSPHRASE"))
	if err != nil {
		log.Fatalf("failed to create event server: %v", err)
	}
	event_service.RegisterEventServiceServer(s, eventServer)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
