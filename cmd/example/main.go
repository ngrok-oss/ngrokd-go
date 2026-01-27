package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	ngrokd "github.com/ishanj12/ngrokd-go"
)

func main() {
	apiKey := os.Getenv("NGROK_API_KEY")
	if apiKey == "" {
		fmt.Println("Usage: NGROK_API_KEY=your-key go run main.go")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Creating ngrokd dialer...")

	// Create dialer with fallback for non-ngrok endpoints
	// Uses FileStore by default (~/.ngrokd-go/certs) to reuse operator across runs
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		APIKey:         apiKey,
		FallbackDialer: &net.Dialer{},
	})
	if err != nil {
		fmt.Printf("Failed to create dialer: %v\n", err)
		os.Exit(1)
	}
	defer dialer.Close()

	fmt.Printf("Operator ID: %s\n", dialer.OperatorID())

	// Discover endpoints
	fmt.Println("\nDiscovering bound endpoints...")
	endpoints, err := dialer.DiscoverEndpoints(ctx)
	if err != nil {
		fmt.Printf("Failed to discover endpoints: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d endpoints:\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Printf("  - %s (%s:%d) [%s]\n", ep.URL, ep.Hostname, ep.Port, ep.Proto)
	}

	if len(endpoints) == 0 {
		fmt.Println("\nNo endpoints found. Create a bound endpoint in ngrok first.")
		fmt.Println("Example: ngrok http 8080 --binding=my-operator")
		return
	}

	// Try to dial the first endpoint
	firstEndpoint := endpoints[0]
	fmt.Printf("\nTrying to dial: %s:%d\n", firstEndpoint.Hostname, firstEndpoint.Port)

	conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", firstEndpoint.Hostname, firstEndpoint.Port))
	if err != nil {
		fmt.Printf("Failed to dial: %v\n", err)
		os.Exit(1)
	}
	
	// Actually test the connection by setting a deadline and reading
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	conn.Close()
	
	if readErr != nil {
		// timeout is expected if backend doesn't send first - that's OK
		if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Connection established (binding upgrade succeeded, read timed out waiting for backend)")
		} else {
			fmt.Printf("Connection established but read failed: %v\n", readErr)
		}
	} else {
		fmt.Println("Connection established and received data from backend!")
	}

	// Test HTTP if it's an HTTP endpoint
	if firstEndpoint.Proto == "http" || firstEndpoint.Proto == "https" {
		fmt.Printf("\nTesting HTTP request to %s...\n", firstEndpoint.URL)

		client := &http.Client{
			Transport: &http.Transport{
				DialContext: dialer.DialContext,
			},
			Timeout: 10 * time.Second,
		}

		resp, err := client.Get(firstEndpoint.URL)
		if err != nil {
			fmt.Printf("HTTP request failed: %v\n", err)
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
			fmt.Printf("Response: %d\n%s\n", resp.StatusCode, string(body))
		}
	}

	// Test fallback dialer
	fmt.Println("\nTesting fallback dialer with httpbin.org...")
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		fmt.Printf("Fallback request failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Fallback response: %d (via standard dialer)\n", resp.StatusCode)
	}

	fmt.Println("\nDone!")
}
