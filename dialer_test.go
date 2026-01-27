package ngrokd

import (
	"context"
	"net"
	"testing"
)

func TestParseAddress(t *testing.T) {
	tests := []struct {
		input    string
		hostname string
		port     int
		wantErr  bool
	}{
		{"my-app.ngrok.app", "my-app.ngrok.app", 443, false},
		{"my-app.ngrok.app:8080", "my-app.ngrok.app", 8080, false},
		{"https://my-app.ngrok.app", "my-app.ngrok.app", 443, false},
		{"http://my-app.ngrok.app", "my-app.ngrok.app", 80, false},
		{"tls://my-app.ngrok.app", "my-app.ngrok.app", 443, false},
		{"https://my-app.ngrok.app:9000", "my-app.ngrok.app", 9000, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hostname, port, err := parseAddress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAddress(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if hostname != tt.hostname {
				t.Errorf("parseAddress(%q) hostname = %v, want %v", tt.input, hostname, tt.hostname)
			}
			if port != tt.port {
				t.Errorf("parseAddress(%q) port = %v, want %v", tt.input, port, tt.port)
			}
		})
	}
}

func TestIsKnownEndpoint(t *testing.T) {
	d := &Dialer{
		endpoints: map[string]Endpoint{
			"my-service.ngrok.app": {Hostname: "my-service.ngrok.app", Port: 443},
		},
	}

	if !d.isKnownEndpoint("my-service.ngrok.app") {
		t.Error("expected my-service.ngrok.app to be known")
	}

	if d.isKnownEndpoint("unknown.example.com") {
		t.Error("expected unknown.example.com to be unknown")
	}
}

// mockDialer records calls and returns a mock connection
type mockDialer struct {
	called  bool
	address string
}

func (m *mockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	m.called = true
	m.address = address
	// Return a pipe for testing (one end of a connection)
	client, _ := net.Pipe()
	return client, nil
}

func TestFallbackDialer(t *testing.T) {
	mock := &mockDialer{}

	d := &Dialer{
		endpoints:      map[string]Endpoint{
			"known.ngrok.app": {Hostname: "known.ngrok.app", Port: 443},
		},
		fallbackDialer: mock,
	}

	// Unknown endpoint should use fallback
	ctx := context.Background()
	conn, err := d.DialContext(ctx, "tcp", "unknown.example.com:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()

	if !mock.called {
		t.Error("expected fallback dialer to be called")
	}
	if mock.address != "unknown.example.com:443" {
		t.Errorf("expected address unknown.example.com:443, got %s", mock.address)
	}
}

func TestNoFallbackReturnsError(t *testing.T) {
	d := &Dialer{
		endpoints: map[string]Endpoint{},
		// No fallback dialer
	}

	ctx := context.Background()
	_, err := d.DialContext(ctx, "tcp", "unknown.example.com:443")
	if err == nil {
		t.Error("expected error for unknown endpoint with no fallback")
	}
}
