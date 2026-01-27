# ngrokd-go-sdk

A Go SDK for programmatic ngrok connectivity, replacing the ngrokd daemon for embedded use cases like routers in ECS Fargate.

## What it does

- Discovers Kubernetes bound endpoints by polling ngrok API
- Creates mTLS connections to ngrok cloud ingress
- Uses binding protocol upgrade for endpoint routing
- Auto-provisions mTLS certificates via CreateKubernetesOperator API
- Provides `net.Dialer`-compatible interface for `http.Transport` integration

## Installation

```bash
go get github.com/ishanjain/ngrokd-go-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "net"
    "net/http"
    "time"

    ngrokd "github.com/ishanjain/ngrokd-go-sdk"
)

func main() {
    ctx := context.Background()

    // Create dialer - auto-provisions and caches mTLS certificate
    dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
        APIKey:         "your-ngrok-api-key",
        FallbackDialer: &net.Dialer{}, // For non-ngrok endpoints
    })
    if err != nil {
        panic(err)
    }
    defer dialer.Close()

    // Discover ngrok-bound endpoints
    endpoints, _ := dialer.DiscoverEndpoints(ctx)
    for _, ep := range endpoints {
        println(ep.URL)
    }

    // Create HTTP client with ngrok-aware transport
    client := &http.Client{
        Transport: &http.Transport{
            DialContext: dialer.DialContext,
        },
    }

    // Requests to ngrok endpoints route through ngrok cloud
    // Other requests use the fallback dialer
    resp, _ := client.Get("https://my-service.ngrok.app/api")
    defer resp.Body.Close()
}
```

## Configuration

```go
ngrokd.Config{
    // Required
    APIKey: "your-api-key",

    // Optional: Route non-ngrok endpoints through standard dialer
    FallbackDialer: &net.Dialer{},

    // Optional: Background endpoint refresh (for long-running services)
    RefreshInterval: 5 * time.Minute,

    // Optional: Re-discover endpoints on cache miss
    RefreshOnMiss: true,

    // Optional: Retry with exponential backoff
    RetryConfig: ngrokd.RetryConfig{
        MaxRetries:     3,
        InitialBackoff: 100 * time.Millisecond,
        MaxBackoff:     5 * time.Second,
    },

    // Optional: Custom certificate storage (default: ~/.ngrokd-go-sdk/certs)
    CertStore: ngrokd.NewFileStore("/custom/path"),
    // Or for ephemeral environments:
    CertStore: ngrokd.NewMemoryStore(),
}
```

## Certificate Storage

The SDK auto-provisions mTLS certificates and stores them for reuse:

| Store | Use Case |
|-------|----------|
| `FileStore` (default) | Persistent environments (EC2, VMs) |
| `MemoryStore` | Ephemeral environments (Fargate, Lambda) |
| Custom `CertStore` | Vault, AWS Secrets Manager, etc. |

## Error Handling

```go
import "errors"

conn, err := dialer.DialContext(ctx, "tcp", "service.ngrok.app:443")
if errors.Is(err, ngrokd.ErrEndpointNotFound) {
    // Endpoint not in cache and no fallback
}
if errors.Is(err, ngrokd.ErrDialFailed) {
    // Network error connecting to ngrok
}
if errors.Is(err, ngrokd.ErrUpgradeFailed) {
    // Binding protocol error
}
if errors.Is(err, ngrokd.ErrClosed) {
    // Dialer was closed
}
```

## Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────────────┐
│  Your App   │────▶│  ngrokd-go-sdk   │────▶│  kubernetes-binding-    │
│             │     │  (DialContext)   │     │  ingress.ngrok.io:443   │
└─────────────┘     └──────────────────┘     └───────────┬─────────────┘
                            │                            │
                            │                            ▼
                    ┌───────▼───────┐           ┌─────────────────┐
                    │  ngrok API    │           │  Your K8s Pod   │
                    │  /bound_      │           │  (via binding)  │
                    │  endpoints    │           └─────────────────┘
                    └───────────────┘
```

## License

MIT
