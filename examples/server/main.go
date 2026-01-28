// Server example: Uses ngrok-go to create an internal agent endpoint
// that forwards traffic to a hello world HTTP server on port 8080.
//
// Usage:
//   NGROK_AUTHTOKEN=your-authtoken go run main.go
//
// This creates an agent endpoint. To connect via the ngrokd-go client,
// you must separately configure a kubernetes-bound cloud endpoint that
// routes to this agent endpoint (via ngrok dashboard or API).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.ngrok.com/ngrok/v2"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start local hello world server on port 8080
	go startHelloServer()

	// Create ngrok agent endpoint
	// This forwards traffic from ngrok to localhost:8080
	fwd, err := ngrok.Forward(ctx,
		ngrok.WithUpstream("http://localhost:8080"),
		ngrok.WithDescription("ngrokd-go example server"),
	)
	if err != nil {
		return fmt.Errorf("failed to create ngrok endpoint: %w", err)
	}

	fmt.Println("===========================================")
	fmt.Println("Internal Agent Endpoint Started")
	fmt.Println("===========================================")
	fmt.Printf("Agent Endpoint URL: %s\n", fwd.URL())
	fmt.Println()
	fmt.Println("To connect via ngrokd-go client:")
	fmt.Println("1. Create a kubernetes-bound cloud endpoint in ngrok")
	fmt.Println("   that routes traffic to this agent endpoint")
	fmt.Println("2. Run the client example to discover and dial it")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println("===========================================")

	<-fwd.Done()
	return nil
}

func startHelloServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "Hello from ngrokd-go example server!")
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Hello server listening on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Hello server error: %v", err)
		os.Exit(1)
	}
}
