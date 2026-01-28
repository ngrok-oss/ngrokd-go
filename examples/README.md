# ngrokd-go Examples

End-to-end examples demonstrating the ngrokd-go SDK.

## Architecture

```
┌─────────────┐                    ┌──────────────────────┐                    ┌─────────────┐
│             │   mTLS + binding   │                      │                    │             │
│   Client    │ ─────────────────► │   ngrok cloud        │                    │   Server    │
│ (ngrokd-go) │     protocol       │                      │                    │  (ngrok-go) │
│             │                    │  ┌────────────────┐  │   agent session    │             │
└─────────────┘                    │  │ K8s-bound      │  │ ◄───────────────── │  Hello:8080 │
      │                            │  │ Cloud Endpoint │──┼────────────────►   │             │
      │                            │  └────────────────┘  │   forwards to      └─────────────┘
      │                            │         │            │   internal agent
      │                            │         ▼            │
      │                            │  ┌────────────────┐  │
      └───────────────────────────►│  │ Binding        │  │
           discovers via API       │  │ Ingress        │  │
                                   │  └────────────────┘  │
                                   └──────────────────────┘
```

**Key components:**

1. **Server (ngrok-go)**: Creates an internal agent endpoint forwarding to localhost:8080
2. **Cloud Endpoint (kubernetes-bound)**: Configured separately in ngrok, routes to the agent
3. **Client (ngrokd-go)**: Discovers kubernetes-bound endpoints via API, dials via binding ingress

## Prerequisites

1. **ngrok account** with API access
2. **NGROK_AUTHTOKEN** for the server
3. **NGROK_API_KEY** for the client
4. A **kubernetes-bound cloud endpoint** configured to route to the agent

Get credentials from https://dashboard.ngrok.com

## Running the Examples

### Step 1: Start the Server

```bash
cd examples/server
NGROK_AUTHTOKEN=your-authtoken go run main.go

# Or with a custom endpoint name:
NGROK_AUTHTOKEN=your-authtoken ENDPOINT_NAME=my-service go run main.go
```

Output:
```
===========================================
Internal Agent Endpoint Started
===========================================
Internal Endpoint URL: https://hello-server.internal
...
```

The `.internal` TLD means this endpoint is only accessible via the kubernetes binding ingress.

### Step 2: Run the Client

```bash
cd examples/client
NGROK_API_KEY=your-api-key go run main.go
```

The client will:
1. Provision an mTLS certificate (stored in `~/.ngrokd-go/certs`)
2. Discover kubernetes-bound endpoints via the ngrok API
3. Dial the endpoint via the binding ingress
4. Make HTTP requests to the hello world server

## How It Works

### Server (ngrok-go)

```go
fwd, err := ngrok.Forward(ctx,
    ngrok.WithUpstream("http://localhost:8080"),
)
```

Creates an agent session and forwards traffic to localhost:8080.

### Client (ngrokd-go)

```go
dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
    APIKey:        apiKey,
    DefaultDialer: &net.Dialer{},
})

endpoints, _ := dialer.DiscoverEndpoints(ctx)

httpClient := &http.Client{
    Transport: &http.Transport{
        DialContext: dialer.DialContext,
    },
}

resp, _ := httpClient.Get(endpoints[0].URL)
```

The SDK:
1. Registers as a kubernetes operator and provisions mTLS cert
2. Polls the ngrok API for kubernetes-bound endpoints
3. Routes matching traffic through binding ingress with binding protocol
4. Non-ngrok traffic passes through to `DefaultDialer`
