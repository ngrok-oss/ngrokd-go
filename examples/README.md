# ngrokd-go Examples

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [ngrok account](https://dashboard.ngrok.com)
- `NGROK_AUTHTOKEN` for the server
- `NGROK_API_KEY` for the client (first run only)

## Examples

### client/

Uses `DiscoveryDialer` with API key to provision cert and list endpoints.

```sh
NGROK_API_KEY=xxxx go run examples/client/main.go
```

### direct/

Uses `Dialer` without API key—loads cert from `~/.ngrokd-go/certs`.

```sh
# Requires cert to already exist (run client/ first to provision)
go run examples/direct/main.go http://my-service.namespace:8080
```

### server/

Creates a private endpoint using [ngrok-go](https://github.com/ngrok/ngrok-go).

```sh
NGROK_AUTHTOKEN=xxxx go run examples/server/main.go
```

## End-to-End Demo

1. **Start server** (creates private endpoint):
   ```sh
   NGROK_AUTHTOKEN=xxxx go run examples/server/main.go
   ```

2. **Run client** (provisions cert, discovers endpoint, connects):
   ```sh
   NGROK_API_KEY=xxxx go run examples/client/main.go
   ```

3. **Run direct** (uses saved cert, no API key needed):
   ```sh
   go run examples/direct/main.go http://hello-server.example
   ```
