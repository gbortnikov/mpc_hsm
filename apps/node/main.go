package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/node/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Флаги командной строки
	port := flag.Int("port", 50051, "gRPC server port")
	nodeID := flag.String("node-id", "", "Node ID (required)")
	partyID := flag.String("party-id", "", "Party ID for MPC operations (required)")

	flag.Parse()

	// Валидация
	if *nodeID == "" {
		*nodeID = fmt.Sprintf("node_%d", *port)
	}
	if *partyID == "" {
		*partyID = fmt.Sprintf("party_%d", *port)
	}

	// Создаём listener
	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	// Создаём gRPC сервер
	grpcServer := grpc.NewServer()

	// Регистрируем MPC Node сервис
	mpcServer := server.NewMPCNodeServer(*nodeID, *partyID)
	pb.RegisterMPCNodeServiceServer(grpcServer, mpcServer)

	// Включаем reflection для отладки (grpcurl, etc.)
	reflection.Register(grpcServer)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down gRPC server...")
		grpcServer.GracefulStop()
	}()

	log.Printf("MPC Node started")
	log.Printf("  Node ID:  %s", *nodeID)
	log.Printf("  Party ID: %s", *partyID)
	log.Printf("  Address:  %s", addr)

	// Запускаем сервер
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
