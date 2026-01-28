# ngrokd-go Examples

This example shows complete end-to-end, private connectivity:

1. **Server** creates a private agent endpoint via the [ngrok-go SDK](https://github.com/ngrok/ngrok-go), serving a hello world web app
2. **Client** discovers the private endpoint and dials into it via the ngrokd-go SDK

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

The server uses [ngrok-go](https://github.com/ngrok/ngrok-go) to create a private agent endpoint.

```sh
NGROK_AUTHTOKEN=xxxx go run examples/server/main.go
```

You should see:
```
Endpoint online: https://hello-server.example
```

### Step 2: Run the Client

The client uses ngrokd-go to discover the private endpoint and dial into it.

```sh
NGROK_API_KEY=xxxx go run examples/client/main.go
```

You should see:
```
Found 1 endpoint(s)
  - https://hello-server.example
Connecting to https://hello-server.example...
  Status: 200
  Body: Hello from ngrokd-go!
```
