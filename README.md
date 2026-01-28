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

This example shows a complete end-to-end flow:

1. **Server** creates an internal agent endpoint (`.internal`)
2. **Cloud Endpoint** forwards traffic to the internal endpoint using `forward-internal`
3. **Client** discovers the kubernetes-bound cloud endpoint and dials into it

### Step 1: Start the Server

The server uses [ngrok-go](https://github.com/ngrok/ngrok-go) to create an internal agent endpoint that forwards to a hello world app.

```sh
NGROK_AUTHTOKEN=xxxx go run examples/server/main.go
```

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"golang.ngrok.com/ngrok/v2"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start hello world server on :8080
	go http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from ngrokd-go!")
	}))

	// Create internal agent endpoint (.internal = only accessible via forward-internal)
	fwd, err := ngrok.Forward(ctx,
		ngrok.WithUpstream("http://localhost:8080"),
		ngrok.WithURL("https://hello-server.internal"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Internal endpoint online:", fwd.URL())
	<-fwd.Done()
}
```

### Step 2: Create a Kubernetes-Bound Cloud Endpoint

Create a cloud endpoint with `kubernetes` binding that forwards traffic to the internal endpoint using the [`forward-internal` action](https://ngrok.com/docs/traffic-policy/actions/forward-internal).

#### Via Dashboard

1. Go to [ngrok Dashboard → Endpoints](https://dashboard.ngrok.com/endpoints)
2. Click **+ New Endpoint**
3. Choose **Cloud Endpoint**
4. Configure:
   - **URL**: `https://hello.example` (or any name you prefer)
   - **Binding**: Select `kubernetes`
5. Add the following **Traffic Policy**:
   ```yaml
   on_http_request:
     - actions:
         - type: forward-internal
           config:
             url: https://hello-server.internal
   ```
6. Click **Create**

This creates a kubernetes-bound endpoint that:
- Is only accessible via the kubernetes binding ingress (not publicly addressable)
- Forwards all traffic to your internal agent endpoint at `https://hello-server.internal`

See [ngrok Cloud Endpoints docs](https://ngrok.com/docs/universal-gateway/cloud-endpoints) for more details.

### Step 3: Run the Client

The client uses ngrokd-go to discover the kubernetes-bound endpoint and dial into it.

```sh
NGROK_API_KEY=xxxx go run examples/client/main.go
```

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	ngrokd "github.com/ishanj12/ngrokd-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create ngrokd dialer (uses NGROK_API_KEY env var)
	dialer, err := ngrokd.NewDialer(ctx, ngrokd.Config{
		DefaultDialer: &net.Dialer{},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer dialer.Close()

	// Discover kubernetes-bound endpoints
	endpoints, _ := dialer.DiscoverEndpoints(ctx)
	log.Printf("Found %d endpoint(s)", len(endpoints))

	// Create HTTP client with ngrokd transport
	httpClient := &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}

	// Connect to discovered endpoints
	for _, ep := range endpoints {
		resp, err := httpClient.Get(ep.URL)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
	}
}
```

## Architecture

```
┌─────────────┐                         ┌──────────────────────────────────────┐                         ┌─────────────┐
│             │      mTLS + binding     │              ngrok cloud             │                         │             │
│   Client    │ ──────────────────────► │                                      │                         │   Server    │
│ (ngrokd-go) │        protocol         │  ┌─────────────────────────────────┐ │      agent session      │  (ngrok-go) │
│             │                         │  │  K8s-bound Cloud Endpoint       │ │ ◄────────────────────── │             │
└─────────────┘                         │  │  https://hello.example          │ │                         │  Hello:8080 │
      │                                 │  │  binding: kubernetes            │ │                         │             │
      │ discovers via                   │  └───────────────┬─────────────────┘ │                         └─────────────┘
      │ ngrok API                       │                  │                   │                               ▲
      │                                 │                  │ forward-internal  │                               │
      │                                 │                  ▼                   │                               │
      │                                 │  ┌─────────────────────────────────┐ │      forwards to              │
      │                                 │  │  Internal Agent Endpoint        │─┼───────────────────────────────┘
      │                                 │  │  https://hello-server.internal  │ │      localhost:8080
      │                                 │  │  binding: internal              │ │
      │                                 │  └─────────────────────────────────┘ │
      │                                 │                                      │
      └────────────────────────────────►│  ┌─────────────────────────────────┐ │
                                        │  │  Kubernetes Binding Ingress     │ │
                                        │  └─────────────────────────────────┘ │
                                        └──────────────────────────────────────┘
```

## Examples

- [Server](./examples/server/) - Create an internal agent endpoint with ngrok-go.
- [Client](./examples/client/) - Discover and dial endpoints with ngrokd-go.

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
