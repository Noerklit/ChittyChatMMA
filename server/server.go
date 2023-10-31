package main

import (
	proto "chittyChat/grpc"
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
)

var lamport int64

type ChittyChatServer struct {
	proto.UnimplementedChittyChatServiceServer
}

var users = make(map[int64]proto.ChittyChatService_JoinChatServer) // map of users

func main() {
	lamport = 0

	//Start the gRPC server
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}
	log.Printf("Starting ChittyChat server on port 50051...")

	//Create an empty gRPC server
	gRPCServer := grpc.NewServer()

	//Create a new instance of the ChittyChatServer and bind it to our empty gRPC server
	ccs := ChittyChatServer{}
	proto.RegisterChittyChatServiceServer(gRPCServer, &ccs)

	err = gRPCServer.Serve(listener)
	if err != nil {
		log.Fatalf("Failed to serve gRPC server on port 50051: %v", err)
	}
}

func (j *ChittyChatServer) JoinChat(user *proto.User, ccsi proto.ChittyChatService_JoinChatServer) error {
	incomingLamport := user.Lamport
	lamport = max(lamport, incomingLamport)
	lamport++

	users[user.Id] = ccsi

	defer func() {
		if err := recover(); err != nil {
			log.Printf("Oh god press the panic button: %v", err)
			os.Exit(1)
		}
	}()

	content := fmt.Sprintf("%s joined the chat", user.Name)

	Broadcast(&proto.FromClient{Name: "ServerMessage", Content: content, Lamport: lamport})

	//Function to block
	bl := make(chan bool)
	<-bl

	return nil
}

func (s *ChittyChatServer) LeaveChat(ctx context.Context, user *proto.User) (*proto.Empty, error) {
	incomingLamport := user.Lamport
	lamport = max(lamport, incomingLamport)
	lamport++

	//Remove the participant
	delete(users, user.Id)

	//Update the timestamp
	Broadcast(&proto.FromClient{Name: "ServerMessage", Content: user.Name + " has left the chat", Lamport: lamport})
	return &proto.Empty{}, nil
}

func (p *ChittyChatServer) PublishMessage(ctx context.Context, msg *proto.FromClient) (*proto.Empty, error) {
	incomingLamport := msg.Lamport
	lamport = max(lamport, incomingLamport)
	lamport++

	msg.Lamport = lamport

	Broadcast(msg)
	return &proto.Empty{}, nil
}

func Broadcast(msg *proto.FromClient) {
	name := msg.Name
	content := msg.Content
	lamport := msg.Lamport

	log.Printf("%s, %s, %d", name, content, lamport)

	for key, value := range users {
		err := value.Send(&proto.FromServer{Name: name, Content: content, Lamport: lamport})
		if err != nil {
			log.Printf("Failed to broadcast message to"+string(key)+": ", err)
			log.Printf("Failed to broadcast message to %s: %v", fmt.Sprint(key), err)
		}
	}
}

func max(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}
