package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

	// Create ngrokd dialer (uses NGROK_API_KEY env var)
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		DefaultDialer: &net.Dialer{},
	})
	if err != nil {
		return err
	}
	defer dialer.Close()

	// Discover kubernetes-bound endpoints (populates dialer cache)
	if _, err := dialer.DiscoverEndpoints(ctx); err != nil {
		return err
	}

	// Create HTTP client with ngrokd transport
	httpClient := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
		Timeout:   30 * time.Second,
	}

	// Dial the server's hardcoded endpoint
	resp, err := httpClient.Get("https://hello-server.example")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))

	return nil
}
