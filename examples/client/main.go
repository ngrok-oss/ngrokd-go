// Client example: Uses ngrokd-go SDK to discover kubernetes-bound endpoints
// and dial into the server example running behind ngrok.
//
// Usage:
//   NGROK_API_KEY=your-api-key go run main.go
//
// Prerequisites:
//   1. Start the server example first (in examples/server)
//   2. The server creates an endpoint with kubernetes binding
//   3. This client discovers that endpoint and connects to it
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	ngrokd "github.com/ishanj12/ngrokd-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apiKey := os.Getenv("NGROK_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("NGROK_API_KEY environment variable is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("===========================================")
	fmt.Println("ngrokd-go Client Example")
	fmt.Println("===========================================")

	// Create ngrokd dialer
	// - Automatically provisions mTLS certificate (stored in ~/.ngrokd-go/certs)
	// - PollingInterval: 30s (background refresh of endpoints)
	// - DefaultDialer: routes non-ngrok traffic to standard dialer
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		APIKey:          apiKey,
		DefaultDialer:   &net.Dialer{},
		PollingInterval: 10 * time.Second, // Poll frequently for demo
		RetryConfig: ngrokd.RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 100 * time.Millisecond,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create dialer: %w", err)
	}
	defer dialer.Close()

	fmt.Printf("Operator ID: %s\n\n", dialer.OperatorID())

	// Discover kubernetes-bound endpoints
	fmt.Println("Discovering kubernetes-bound endpoints...")
	endpoints, err := dialer.DiscoverEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		fmt.Println()
		fmt.Println("No endpoints found!")
		fmt.Println()
		fmt.Println("Make sure the server example is running:")
		fmt.Println("  cd examples/server && NGROK_AUTHTOKEN=xxx go run main.go")
		return nil
	}

	fmt.Printf("Found %d endpoint(s):\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Printf("  - %s (proto: %s)\n", ep.URL, ep.Proto)
	}
	fmt.Println()

	// Create HTTP client using ngrokd dialer
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
		Timeout: 30 * time.Second,
	}

	// Connect to each discovered endpoint
	for _, ep := range endpoints {
		fmt.Printf("Connecting to %s...\n", ep.URL)

		resp, err := httpClient.Get(ep.URL)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		fmt.Printf("  Status: %d\n", resp.StatusCode)
		fmt.Printf("  Body: %s\n", string(body))

		// Also try the /health endpoint
		healthURL := ep.URL + "/health"
		resp, err = httpClient.Get(healthURL)
		if err != nil {
			fmt.Printf("  Health check error: %v\n\n", err)
			continue
		}

		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  Health: %s\n\n", string(body))
	}

	fmt.Println("===========================================")
	fmt.Println("Done!")
	fmt.Println("===========================================")

	return nil
}
