# ngrokd-go Examples

This example shows complete end-to-end, private connectivity:

1. **Server** creates an internal agent endpoint (`.internal`) via the [ngrok-go SDK](https://github.com/ngrok/ngrok-go/tree/main), serving a local hello world web app running on port 8080
2. **Private Cloud Endpoint** forwards traffic to the internal agent endpoint using the `forward-internal` traffic policy action
3. **Client** discovers the private cloud endpoint and dials into it

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [ngrok account](https://dashboard.ngrok.com)
- `NGROK_AUTHTOKEN` for the server
- `NGROK_API_KEY` for the client

## Running the Examples

First, clone the repository:

```sh
git clone https://github.com/ishanj12/ngrokd-go.git
cd ngrokd-go
```

### Step 1: Start the Server

The server uses [ngrok-go](https://github.com/ngrok/ngrok-go) to create an internal agent endpoint that forwards to a hello world app.

```sh
NGROK_AUTHTOKEN=xxxx go run examples/server/main.go
```

You should see:
```
Internal endpoint online: https://hello-server.internal
```

### Step 2: Create a Private Cloud Endpoint

Create a cloud endpoint with `kubernetes` binding that forwards traffic to the internal endpoint using the [`forward-internal` action](https://ngrok.com/docs/traffic-policy/actions/forward-internal).

**Note**: This is denoted as a kubernetes binding because this SDK was built using ngrok's Kubernetes Operator foundational logic. However, it is meant to run in **non-Kubernetes** environments.

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

This creates a private endpoint that:
- Is only accessible from your local application running the ngrokd-go SDK, and is not publicly addressable on the internet
- Forwards all traffic to your internal agent endpoint at `https://hello-server.internal`

See [ngrok Cloud Endpoints docs](https://ngrok.com/docs/universal-gateway/cloud-endpoints) for more details.

### Step 3: Run the Client

The client uses ngrokd-go to discover the private endpoint and dial into it.

```sh
NGROK_API_KEY=xxxx go run examples/client/main.go
```

You should see:
```
Operator ID: op_xxx
Found 1 endpoint(s)
  - https://hello.example
Connecting to https://hello.example...
  Status: 200
  Body: Hello from ngrokd-go!
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
