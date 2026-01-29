# ngrokd-go

[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/ishanj12/ngrokd-go/blob/main/LICENSE)

A Go SDK for connecting to remote services via ngrok's private endpoints. Instead of running the [ngrokd daemon](https://ngrokd.ngrok.app/), embed this library directly in your Go application.

ngrokd-go enables you to dial into private ngrok endpoints from anywhere. It handles mTLS certificate provisioning, endpoint discovery, and the binding protocol automatically.

## Installation

```sh
go get github.com/ishanj12/ngrokd-go
```

## Quickstart

```go
package main

import (
	"context"
	"net/http"

	ngrokd "github.com/ishanj12/ngrokd-go"
)

func main() {
	ctx := context.Background()

	// Create ngrokd dialer 
	dialer, _ := ngrokd.NewDialer(ctx, ngrokd.Config{})
	defer dialer.Close()

	// Create HTTP client that routes through ngrok
	client := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}

	// Make requests to private endpoints
	resp, _ := client.Get("https://my-service.example")
}
```

See [examples/](./examples/) for a complete end-to-end demo with server and client.

## Configuration

```go
ngrokd.Config{
	// Required: ngrok API key (or set NGROK_API_KEY env var)
	APIKey: "your-api-key",

	// Routes non-ngrok traffic to standard dialer (default: &net.Dialer{})
	DefaultDialer: &net.Dialer{},

	// Background endpoint refresh interval (default: 30s)
	PollingInterval: 30 * time.Second,

	// Filter endpoints with CEL expressions (default: all)
	EndpointSelectors: []string{"endpoint.metadata.name == 'my-service'"},
}
```

## Certificate Storage

Certificates are cached to avoid re-provisioning on restart:

- `FileStore` (default) - Saves to `~/.ngrokd-go/certs`
- `MemoryStore` - For ephemeral environments like Fargate or Lambda

## Documentation

- [Examples](./examples/) - Complete end-to-end demo
- [ngrok Documentation](https://ngrok.com/docs)

## License

MIT
