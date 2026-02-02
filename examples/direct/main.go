package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	ngrokd "github.com/ngrok-oss/ngrokd-go"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: direct <endpoint-url>")
	}

	// Auto-loads cert from ~/.ngrokd-go/certs
	dialer, err := ngrokd.Dialer(ngrokd.DirectConfig{})
	if err != nil {
		log.Fatalf("Failed to create dialer: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
		Timeout:   30 * time.Second,
	}

	target := os.Args[1]
	log.Printf("Connecting to %s...", target)

	resp, err := client.Get(target)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
}
