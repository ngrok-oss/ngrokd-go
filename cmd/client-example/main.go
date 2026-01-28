package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	ngrokd "github.com/ishanj12/ngrokd-go"
)

// ============================================================
// This is a simplified version of your OpenAPI client pattern
// ============================================================

// HttpRequestDoer performs HTTP requests.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// RequestEditorFn is a callback for modifying requests
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Client which conforms to the OpenAPI3 specification for this service.
type Client struct {
	Server         string
	Client         HttpRequestDoer
	RequestEditors []RequestEditorFn
}

// ClientOption allows setting custom parameters during construction
type ClientOption func(*Client) error

// Creates a new Client, with reasonable defaults
func NewClient(server string, opts ...ClientOption) (*Client, error) {
	client := Client{
		Server: server,
	}
	for _, o := range opts {
		if err := o(&client); err != nil {
			return nil, err
		}
	}
	if !strings.HasSuffix(client.Server, "/") {
		client.Server += "/"
	}
	if client.Client == nil {
		client.Client = &http.Client{}
	}
	return &client, nil
}

// WithHTTPClient allows overriding the default Doer
func WithHTTPClient(doer HttpRequestDoer) ClientOption {
	return func(c *Client) error {
		c.Client = doer
		return nil
	}
}

// Example method on the client
func (c *Client) GetHealth(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"health", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("Status: %d, Body: %s", resp.StatusCode, string(body)), nil
}

// ============================================================
// Main: Integrate ngrokd SDK with the OpenAPI client
// ============================================================

func main() {
	apiKey := os.Getenv("NGROK_API_KEY")
	if apiKey == "" {
		fmt.Println("Usage: NGROK_API_KEY=your-key go run main.go")
		os.Exit(1)
	}

	// Target endpoint - this should be a kubernetes-bound endpoint
	targetServer := os.Getenv("TARGET_SERVER")
	if targetServer == "" {
		targetServer = "http://sdk.test" // default to your test endpoint
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Creating ngrokd dialer...")

	// Create ngrokd dialer
	// - RefreshInterval defaults to 30 seconds (polls ngrok API for endpoints)
	// - RetryConfig: Retry transient failures with exponential backoff
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		APIKey:         apiKey,
		FallbackDialer: &net.Dialer{}, // Use standard dialer for non-ngrok endpoints
		RetryConfig: ngrokd.RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     5 * time.Second,
		},
	})
	if err != nil {
		fmt.Printf("Failed to create dialer: %v\n", err)
		os.Exit(1)
	}
	defer dialer.Close()

	// Discover endpoints so the dialer knows which hosts are ngrok endpoints
	endpoints, err := dialer.DiscoverEndpoints(ctx)
	if err != nil {
		fmt.Printf("Failed to discover endpoints: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Discovered %d ngrok endpoints:\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Printf("  - %s\n", ep.URL)
	}

	// Create HTTP client with ngrokd transport
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
		Timeout: 30 * time.Second,
	}

	// Create your OpenAPI client with the ngrokd-enabled HTTP client
	client, err := NewClient(
		targetServer,
		WithHTTPClient(httpClient),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMaking request to %s...\n", targetServer)

	// Make a request - this will route through ngrok if targetServer is a known endpoint
	result, err := client.GetHealth(ctx)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Response: %s\n", result)
	fmt.Println("\nDone!")
}
