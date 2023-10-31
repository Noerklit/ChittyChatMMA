package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	proto "chittyChat/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var lamport int64

type Client struct {
	stream     proto.ChittyChatService_JoinChatClient
	Id         int64
	clientName string
}

func main() {
	lamport = 0

	const serverID = "localhost:50051"

	// Set up a connection to the server.
	log.Println("Connecting to: " + serverID)
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("Could not connect to gRPC server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewChittyChatServiceClient(conn)

	ch := Client{}
	ch.clientConfig()

	rand.Seed(time.Now().UnixNano())
	ch.Id = rand.Int63n(1000000)

	lamport++
	var user = proto.User{
		Id:      ch.Id,
		Name:    ch.clientName,
		Lamport: lamport,
	}

	// Create a stream context
	_stream, err := client.JoinChat(context.Background(), &user)
	if err != nil {
		log.Fatalf("Failed to get a response from the server: %v", err)
	}

	ch.stream = _stream

	SetupCloseHandler(ch, client)

	go ch.sendMessage(client)
	go ch.receiveMessage()

	//Block to block main
	bl := make(chan bool)
	<-bl
}

// Function to assign a name to the client
func (ch *Client) clientConfig() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your name: ")
	msg, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read client name from console: %v", err)
	}
	//Remove the carriage return \r and newline characters \n
	ch.clientName = strings.TrimRight(msg, "\r\n")
}

func (ch *Client) sendMessage(client proto.ChittyChatServiceClient) {
	for {
		reader := bufio.NewReader(os.Stdin)
		clientMessage, err := reader.ReadString('\n')
		clientMessage = strings.TrimRight(clientMessage, "\r\n")
		if err != nil {
			log.Fatalf("Failed to read client message from console: %v", err)
			continue
		}
		if len(clientMessage) > 0 && len(clientMessage) <= 128 {
			lamport++
			msg := proto.FromClient{
				Name:    ch.clientName,
				Content: clientMessage,
				Lamport: lamport,
			}
			_, err := client.PublishMessage(context.Background(), &msg)
			if err != nil {
				log.Printf("Error whilst sending message to server: %v", err)
			}

			time.Sleep(450 * time.Millisecond)
		} else {
			log.Printf("Only messages with a length between 1 and 128 characters are valid")
		}
	}
}

func (ch *Client) receiveMessage() {
	for {
		resp, err := ch.stream.Recv()
		if err != nil {
			log.Printf("Could not receive: %v", err)
		}

		incomingLamport := resp.Lamport
		lamport = max(lamport, incomingLamport)
		lamport++

		log.Printf("%s: %s, %d", resp.Name, resp.Content, resp.Lamport)
	}
}

func SetupCloseHandler(ch Client, client proto.ChittyChatServiceClient) {
	cl := make(chan os.Signal)
	signal.Notify(cl, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cl
		fmt.Println("\nClosing connection to server...")
		client.LeaveChat(context.Background(), &proto.User{Id: ch.Id, Name: ch.clientName, Lamport: lamport})
		os.Exit(0)
	}()
}

func max(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}
