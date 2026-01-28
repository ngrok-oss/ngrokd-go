# ngrokd-go

[![Go Reference](https://pkg.go.dev/badge/github.com/ishanj12/ngrokd-go.svg)](https://pkg.go.dev/github.com/ishanj12/ngrokd-go)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/ishanj12/ngrokd-go/blob/main/LICENSE)

A Go SDK for connecting to services via ngrok's kubernetes-bound endpoints. Instead of running the [ngrokd daemon](https://ngrokd.ngrok.app/), embed this library directly in your Go application.

ngrokd-go enables you to dial into private ngrok endpoints from anywhere. It handles mTLS certificate provisioning, endpoint discovery, and the binding protocol automatically.

## Installation

Install ngrokd-go with `go get`.

```sh
go get github.com/ishanj12/ngrokd-go
```

## Documentation

- [Examples](./examples) are a great way to get started.
- [ngrok Documentation](https://ngrok.com/docs) for what you can do with ngrok.

## Quickstart

The following example discovers kubernetes-bound endpoints and lists them.

You need an ngrok API key to run this example, which you can get from the [ngrok dashboard](https://dashboard.ngrok.com/api).

Run this example with the following command:

```sh
NGROK_API_KEY=xxxx go run examples/basic/main.go
```

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
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

	// Create ngrokd dialer using NGROK_API_KEY environment variable
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		DefaultDialer: &net.Dialer{},
	})
	if err != nil {
		return err
	}
	defer dialer.Close()

	log.Println("Operator ID:", dialer.OperatorID())

	// Discover kubernetes-bound endpoints
	endpoints, err := dialer.DiscoverEndpoints(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d endpoint(s):\n", len(endpoints))
	for _, ep := range endpoints {
		fmt.Printf("  - %s (%s)\n", ep.URL, ep.Proto)
	}

	return nil
}
```

## HTTP Client

Use the dialer with any HTTP client by plugging it into `http.Transport`:

```go
dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
	DefaultDialer: &net.Dialer{},
})
if err != nil {
	return err
}
defer dialer.Close()

// Discover endpoints first
endpoints, _ := dialer.DiscoverEndpoints(ctx)

// Create HTTP client with ngrokd transport
httpClient := &http.Client{
	Transport: &http.Transport{DialContext: dialer.DialContext},
}

// Requests to discovered endpoints route through ngrok
// Other requests use DefaultDialer
resp, err := httpClient.Get(endpoints[0].URL)
```

## Examples

- [Basic endpoint discovery](./examples/basic/) - List kubernetes-bound endpoints.
- [HTTP client](./examples/http-client/) - Make HTTP requests to discovered endpoints.

## Configuration

```go
ngrokd.Config{
	// Required: ngrok API key (or set NGROK_API_KEY env var)
	APIKey: "your-api-key",

	// Routes non-ngrok traffic to this dialer
	DefaultDialer: &net.Dialer{},

	// Background endpoint refresh interval (default: 30s)
	PollingInterval: 30 * time.Second,

	// Retry transient failures
	RetryConfig: ngrokd.RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
	},

	// Filter endpoints with CEL expressions (default: all)
	EndpointSelectors: []string{"endpoint.metadata.name == 'my-service'"},
}
```

## Certificate Storage

Certificates are cached to avoid re-provisioning on restart:

- `FileStore` (default) - Saves to `~/.ngrokd-go/certs`
- `MemoryStore` - For ephemeral environments like Fargate or Lambda

## License

MIT
