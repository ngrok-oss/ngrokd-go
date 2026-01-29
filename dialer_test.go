package ngrokd

import (
	"context"
	"net"
	"net/url"
	"testing"
)

func TestParseAddress(t *testing.T) {
	// Test cases for private endpoint URL format: [http|tcp|tls]://name.namespace[:port]
	tests := []struct {
		input    string
		hostname string
		port     int
		wantErr  bool
	}{
		{"app.example", "app.example", 80, false},
		{"app.example:8080", "app.example", 8080, false},
		{"http://app.example", "app.example", 80, false},
		{"http://app.example:9000", "app.example", 9000, false},
		{"tcp://app.example:443", "app.example", 443, false},
		{"tcp://app.example", "", 0, true}, // tcp requires port
		{"tls://app.example:443", "app.example", 443, false},
		{"tls://app.example", "", 0, true}, // tls requires port
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
			"app.example": {ID: "ep_123", URL: mustParseURL("http://app.example")},
		},
	}

	if !d.isKnownEndpoint("app.example") {
		t.Error("expected app.example to be known")
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
		endpoints: map[string]Endpoint{
			"known.example": {ID: "ep_456", URL: mustParseURL("http://known.example")},
		},
		defaultDialer: mock,
	}

	// Unknown endpoint should use fallback
	ctx := context.Background()
	conn, err := d.DialContext(ctx, "tcp", "unknown.example:80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()

	if !mock.called {
		t.Error("expected fallback dialer to be called")
	}
	if mock.address != "unknown.example:80" {
		t.Errorf("expected address unknown.example:80, got %s", mock.address)
	}
}

func TestNoFallbackReturnsError(t *testing.T) {
	d := &Dialer{
		endpoints: map[string]Endpoint{},
		// No fallback dialer
	}

	ctx := context.Background()
	_, err := d.DialContext(ctx, "tcp", "unknown.example:80")
	if err == nil {
		t.Error("expected error for unknown endpoint with no fallback")
	}
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
