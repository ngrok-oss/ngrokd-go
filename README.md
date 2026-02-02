# ngrokd-go

A Go SDK for connecting to remote services via ngrok's private kubernetes-bound endpoints. Embed this library directly in your Go application instead of running the ngrok daemon.

## Installation

```sh
go get github.com/ngrok-oss/ngrokd-go
```

## Two Dialers

### Dialer

No API key required. Uses an existing cert/private key to authenticate with ngrok and dial endpoints directly. Auto-loads from `~/.ngrokd-go/certs` if no cert provided.

- No provisioning, no discovery, no API calls
- Just mTLS dial to ngrok ingress
- Ngrok rejects unknown endpoints with `ERR_NGROK_706`

```go
// Auto-load cert from default location
dialer, err := ngrokd.Dialer(ngrokd.DirectConfig{})

// Or provide cert explicitly
cert, _ := tls.LoadX509KeyPair("tls.crt", "tls.key")
dialer, err := ngrokd.Dialer(ngrokd.DirectConfig{
    Cert: cert,
})

// Use with http.Client
client := &http.Client{
    Transport: &http.Transport{DialContext: dialer.DialContext},
}
resp, _ := client.Get("http://my-service.namespace:8080")
```

### DiscoveryDialer

Provisions certificates via API and provides endpoint visibility.

```go
dialer, err := ngrokd.DiscoveryDialer(ctx, ngrokd.Config{
    APIKey: os.Getenv("NGROK_API_KEY"),
})

// List available endpoints
endpoints, _ := dialer.Endpoints(ctx)
for _, ep := range endpoints {
    fmt.Println(ep.URL)
}

// Dial like normal
client := &http.Client{
    Transport: &http.Transport{DialContext: dialer.DialContext},
}
```

## Workflow

1. **First time**: Use `DiscoveryDialer` with API key to provision certificate (saved to `~/.ngrokd-go/certs`)
2. **After that**: Use `Dialer` without API key—auto-loads saved certificate

```go
// One-time provisioning
d, _ := ngrokd.DiscoveryDialer(ctx, ngrokd.Config{APIKey: "..."})
// Cert is now saved to ~/.ngrokd-go/certs

// Later, no API key needed
d, _ := ngrokd.Dialer(ngrokd.DirectConfig{})
```

## Configuration

### DirectConfig (for Dialer)

```go
ngrokd.DirectConfig{
    Cert:      tls.Certificate{},  // Optional: explicit cert
    CertStore: ngrokd.NewFileStore("/custom/path"),  // Optional: custom storage
}
```

### Config (for DiscoveryDialer)

```go
ngrokd.Config{
    APIKey:            "your-api-key",  // Required
    EndpointSelectors: []string{"true"},  // CEL expressions to filter endpoints
}
```

## Certificate Storage

Certificates are stored to avoid re-provisioning:

- `FileStore` (default) — `~/.ngrokd-go/certs`
- `MemoryStore` — ephemeral, for Lambda/Fargate
- Custom — implement `CertStore` interface

## Examples

See [examples/](./examples/) for complete demos.

## License

MIT
