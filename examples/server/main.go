package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"golang.ngrok.com/ngrok/v2"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	// Create kubernetes-bound agent endpoint
	ln, err := ngrok.Listen(ctx,
		ngrok.WithURL("https://hello-server.example"),
		ngrok.WithBindings("kubernetes"),
		ngrok.WithDescription("ngrokd-go example server"),
	)
	if err != nil {
		return fmt.Errorf("failed to create ngrok endpoint: %w", err)
	}

	log.Println("Endpoint online:", ln.URL())

	// Serve hello world
	return http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		fmt.Fprintln(w, "Hello from ngrokd-go!")
	}))
}
