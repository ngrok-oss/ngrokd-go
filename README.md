# ngrokd-go

A Go SDK for connecting to remote services from anywhere via **private** ngrok endpoints. Instead of running the [ngrokd daemon](https://ngrokd.ngrok.app/), embed this library directly in your Go application.

## What It Does

This SDK lets your Go application connect to services exposed through ngrok's Kubernetes bindings. It works by:

1. **Discovering endpoints** - Polls the ngrok API to learn which hostnames are ngrok-bound
2. **Establishing mTLS connections** - Connects to ngrok's cloud service with a client certificate via mTLS
3. **Routing intelligently** - ngrok traffic uses the ngrokd dialer, everything else uses your fallback dialer
4. **Providing a DialContext** - Plug into `http.Transport` to make any HTTP client ngrok-aware

The SDK provisions its own mTLS certificate by generating a private key locally and having ngrok sign it. The private key never leaves your machine.

## Installation

```bash
go get github.com/ishanj12/ngrokd-go
```

## Usage

```go
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
    ctx := context.Background()

    // Create ngrok-aware dialer
    dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
        APIKey:         os.Getenv("NGROK_API_KEY"),
        FallbackDialer: &net.Dialer{},
    })
    if err != nil {
        panic(err)
    }
    defer dialer.Close()

    // Discover ngrok-bound endpoints
    endpoints, _ := dialer.DiscoverEndpoints(ctx)
    fmt.Printf("Found %d endpoints\n", len(endpoints))
    for _, ep := range endpoints {
        fmt.Printf("  - %s\n", ep.URL)
    }

    // Create HTTP client with ngrok transport
    httpClient := &http.Client{
        Transport: &http.Transport{DialContext: dialer.DialContext},
        Timeout:   10 * time.Second,
    }

    // Make request - routes through ngrok if endpoint is in cache
    if len(endpoints) > 0 {
        resp, err := httpClient.Get(endpoints[0].URL)
        if err != nil {
            panic(err)
        }
        defer resp.Body.Close()
        body, _ := io.ReadAll(resp.Body)
        fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
    }
}
```

## Configuration

```go
ngrokd.Config{
    APIKey:          "your-api-key",
    FallbackDialer:  &net.Dialer{},       // handles non-ngrok traffic
    RefreshInterval: 30 * time.Second,    // background endpoint refresh (default)
    RetryConfig: ngrokd.RetryConfig{
        MaxRetries:     3,
        InitialBackoff: 100 * time.Millisecond,
    },

    // Limit which endpoints this operator can access (default: all)
    EndpointSelectors: []string{"endpoint.metadata.name == 'my-service'"},
}
```

## Certificate Storage

Certificates are cached locally to avoid re-provisioning on restart:

- `FileStore` (default) - Saves to `~/.ngrokd-go/certs`
- `MemoryStore` - For ephemeral environments like Fargate or Lambda

## License

MIT
