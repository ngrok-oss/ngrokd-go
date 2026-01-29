package main

import (
	"context"
	"fmt"
	"io"
	"log"
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

	// Create ngrokd dialer 
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{})
	if err != nil {
		return err
	}
	defer dialer.Close()

	// Create HTTP client using ngrokd dialer
	httpClient := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
		Timeout:   30 * time.Second,
	}

	// Dial the private endpoint started by the server
	log.Println("Connecting to http://hello-server.example...")
	resp, err := httpClient.Get("http://hello-server.example")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))

	return nil
}
