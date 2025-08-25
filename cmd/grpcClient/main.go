package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/setavenger/blindbit-lib/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, conn := NewClient(ctx, "127.0.0.1:8001")
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			panic(err)
		}
	}(conn)

	info, err := client.GetInfo(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", info)

	resp, err := client.GetTweakArray(ctx,
		&pb.BlockHeightRequest{BlockHeight: 240_000},
	)
	if err != nil {
		log.Fatal(err)
	}

	tweaks := resp.GetTweaks()
	for i := range tweaks {
		fmt.Printf("%x\n", tweaks[i])
	}
}

func NewClient(ctx context.Context, host string) (pb.OracleServiceClient, *grpc.ClientConn) {
	// Connect to the server with a timeout context
	conn, err := grpc.DialContext(
		ctx,
		host,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	client := pb.NewOracleServiceClient(conn)
	return client, conn
}
