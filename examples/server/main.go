// Server example: Uses ngrok-go to create an internal agent endpoint
// that forwards traffic to a hello world HTTP server on port 8080.
//
// Usage:
//   NGROK_AUTHTOKEN=your-authtoken go run main.go
//   NGROK_AUTHTOKEN=your-authtoken ENDPOINT_NAME=my-service go run main.go
//
// This creates an internal endpoint (*.internal) that is only accessible
// via the kubernetes binding ingress using the ngrokd-go client.
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

	// Endpoint name defaults to "hello-server" but can be overridden
	endpointName := os.Getenv("ENDPOINT_NAME")
	if endpointName == "" {
		endpointName = "hello-server"
	}

	// Internal endpoints use the .internal TLD
	internalURL := fmt.Sprintf("https://%s.internal", endpointName)

	// Create internal agent endpoint
	fwd, err := ngrok.Forward(ctx,
		ngrok.WithUpstream("http://localhost:8080"),
		ngrok.WithURL(internalURL),
		ngrok.WithDescription("ngrokd-go example server"),
	)
	if err != nil {
		return fmt.Errorf("failed to create ngrok endpoint: %w", err)
	}

	fmt.Println("===========================================")
	fmt.Println("Internal Agent Endpoint Started")
	fmt.Println("===========================================")
	fmt.Printf("Internal Endpoint URL: %s\n", fwd.URL())
	fmt.Println()
	fmt.Println("This endpoint is only accessible via the")
	fmt.Println("kubernetes binding ingress (ngrokd-go client).")
	fmt.Println()
	fmt.Println("Run the client example to connect:")
	fmt.Println("  cd examples/client && NGROK_API_KEY=xxx go run main.go")
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
