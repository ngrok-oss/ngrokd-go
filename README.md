# ngrokd-gosdk

A programmatic Go implementation of the [ngrokd daemon](https://ngrok.com/docs/k8s/) for environments where running a sidecar isn't practical—like ECS Fargate, Lambda, or embedded systems.

## How ngrokd Works

The ngrokd daemon creates local TCP listeners and proxies traffic to ngrok-bound Kubernetes endpoints via mTLS:

```
Your App → localhost:PORT → ngrokd → mTLS → ngrok cloud → K8s Pod
```

This SDK eliminates the middleman by providing a `DialContext` that connects directly:

```
Your App → SDK.DialContext() → mTLS → ngrok cloud → K8s Pod
```

## Core Concepts

### 1. Dialer

The SDK is a `net.Dialer` replacement. Plug it into `http.Transport` and existing HTTP code works unchanged:

```go
dialer, _ := ngrokd.NewDialer(ctx, ngrokd.Config{
    APIKey: "your-api-key",
})

client := &http.Client{
    Transport: &http.Transport{
        DialContext: dialer.DialContext,
    },
}

// Routes through ngrok if hostname is a bound endpoint
resp, _ := client.Get("https://my-service.ngrok.app/api")
```

### 2. mTLS Certificate Provisioning

The SDK provisions mTLS certificates the same way the ngrok Kubernetes Operator does:

1. **Generate** ECDSA P-384 private key locally (never leaves your machine)
2. **Create** Certificate Signing Request (CSR)
3. **Send** CSR to ngrok API (`POST /kubernetes_operators`)
4. **Receive** signed certificate from ngrok
5. **Store** key + cert for reuse

```go
// Certificate storage backends
ngrokd.NewFileStore("/path/to/certs")     // Persistent (default: ~/.ngrokd-gosdk/certs)
ngrokd.NewMemoryStore()                    // Ephemeral (Fargate, Lambda)
ngrokd.NewMemoryStoreWithCert(key, cert, opID)  // Pre-loaded from Secrets Manager
```

### 3. Fallback Dialer

Not all traffic goes through ngrok. The SDK checks its endpoint cache and falls back to a standard dialer for non-ngrok hosts:

```go
dialer, _ := ngrokd.NewDialer(ctx, ngrokd.Config{
    APIKey:         "your-api-key",
    FallbackDialer: &net.Dialer{},  // Standard dialer for non-ngrok endpoints
})

// ngrok endpoint → routes through ngrok cloud
client.Get("https://my-service.ngrok.app/api")

// Regular endpoint → uses FallbackDialer
client.Get("https://api.stripe.com/v1/charges")
```

## Installation

```bash
go get github.com/ishanj12/ngrokd-gosdk
```

## Quick Start

```go
package main

import (
    "context"
    "net"
    "net/http"

    ngrokd "github.com/ishanj12/ngrokd-gosdk"
)

func main() {
    ctx := context.Background()

    dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
        APIKey:         "your-ngrok-api-key",
        FallbackDialer: &net.Dialer{},
    })
    if err != nil {
        panic(err)
    }
    defer dialer.Close()

    // Discover ngrok-bound endpoints
    dialer.DiscoverEndpoints(ctx)

    // Create HTTP client
    client := &http.Client{
        Transport: &http.Transport{DialContext: dialer.DialContext},
    }

    // Use normally - SDK handles routing
    resp, _ := client.Get("https://my-k8s-service.ngrok.app/health")
    defer resp.Body.Close()
}
```

## Production Configuration

```go
ngrokd.Config{
    APIKey:         "your-api-key",
    FallbackDialer: &net.Dialer{},

    // Background refresh for long-running services
    RefreshInterval: 5 * time.Minute,

    // Re-discover on cache miss (handles new endpoints)
    RefreshOnMiss: true,

    // Retry transient failures
    RetryConfig: ngrokd.RetryConfig{
        MaxRetries:     3,
        InitialBackoff: 100 * time.Millisecond,
        MaxBackoff:     5 * time.Second,
    },
}
```

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         Your Application                         │
│                                                                  │
│   http.Client{Transport: &http.Transport{DialContext: dialer}}  │
└─────────────────────────────┬────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                       ngrokd-gosdk Dialer                        │
│                                                                  │
│  ┌─────────────┐    ┌──────────────┐    ┌────────────────────┐  │
│  │  Endpoint   │    │    mTLS      │    │   CertStore        │  │
│  │   Cache     │    │  Connection  │    │  (File/Memory)     │  │
│  └─────────────┘    └──────────────┘    └────────────────────┘  │
│         │                  │                      │              │
│         ▼                  ▼                      ▼              │
│  ┌─────────────┐    ┌──────────────┐    ┌────────────────────┐  │
│  │ ngrok API   │    │   Binding    │    │  Private Key       │  │
│  │ /bound_     │    │   Protocol   │    │  (never leaves     │  │
│  │ endpoints   │    │   Upgrade    │    │   your machine)    │  │
│  └─────────────┘    └──────────────┘    └────────────────────┘  │
└──────────────────────────────┬───────────────────────────────────┘
                               │
           ┌───────────────────┴───────────────────┐
           ▼                                       ▼
┌─────────────────────┐                 ┌─────────────────────┐
│  ngrok cloud        │                 │  Fallback Dialer    │
│  (mTLS + binding)   │                 │  (standard TCP)     │
│         │           │                 │         │           │
│         ▼           │                 │         ▼           │
│  ┌─────────────┐    │                 │  ┌─────────────┐    │
│  │  K8s Pod    │    │                 │  │  External   │    │
│  │  (your svc) │    │                 │  │  APIs       │    │
│  └─────────────┘    │                 │  └─────────────┘    │
└─────────────────────┘                 └─────────────────────┘
```

## Error Handling

```go
if errors.Is(err, ngrokd.ErrEndpointNotFound) {
    // Hostname not in cache, no fallback configured
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

## ngrokd Daemon vs SDK

| Aspect | ngrokd Daemon | This SDK |
|--------|---------------|----------|
| Runs as | Sidecar process | Embedded library |
| Connection | App → localhost → ngrokd → ngrok | App → SDK → ngrok |
| Storage | Hardcoded file paths | Pluggable CertStore |
| Integration | Config change (point to localhost) | Code change (inject dialer) |
| Best for | VMs, bare metal | Containers, serverless |

## License

MIT
