package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	ngrokd "github.com/ishanj12/ngrokd-go"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Create ngrokd dialer using NGROK_API_KEY environment variable
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		DefaultDialer: &net.Dialer{},
	})
	if err != nil {
		return err
	}
	defer dialer.Close()

	log.Println("Operator ID:", dialer.OperatorID())

	// Discover kubernetes-bound endpoints
	endpoints, err := dialer.DiscoverEndpoints(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d endpoint(s):\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Printf("  - %s (%s)\n", ep.URL, ep.Proto)
	}

	return nil
}
