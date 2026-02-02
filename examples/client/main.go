package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	ngrokd "github.com/ngrok-oss/ngrokd-go"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Create ngrokd discovery dialer with API key (from env: NGROK_API_KEY)
	dialer, err := ngrokd.DiscoveryDialer(ctx, ngrokd.Config{
		APIKey: os.Getenv("NGROK_API_KEY"),
	})
	if err != nil {
		return fmt.Errorf("failed to create dialer: %w", err)
	}

	log.Printf("Operator ID: %s", dialer.OperatorID())

	// List available endpoints
	endpoints, err := dialer.Endpoints(ctx)
	if err != nil {
		return fmt.Errorf("failed to list endpoints: %w", err)
	}

	log.Printf("Found %d endpoints", len(endpoints))
	for _, ep := range endpoints {
		log.Printf("  - %s", ep.URL)
	}

	if len(endpoints) == 0 {
		log.Println("No endpoints found")
		return nil
	}

	// Create HTTP client using ngrokd dialer
	httpClient := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
		Timeout:   30 * time.Second,
	}

	// Use first discovered endpoint, or one from command line
	target := endpoints[0].URL.String()
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	log.Printf("Connecting to %s...", target)
	resp, err := httpClient.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))

	return nil
}
