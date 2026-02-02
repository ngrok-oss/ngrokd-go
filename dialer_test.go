package ngrokd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"
	"time"
)

func TestParseAddress(t *testing.T) {
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
		{"tcp://app.example", "", 0, true},
		{"tls://app.example:443", "app.example", 443, false},
		{"tls://app.example", "", 0, true},
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

func TestDiscoveryDialerRequiresAPIKey(t *testing.T) {
	ctx := context.Background()
	_, err := DiscoveryDialer(ctx, Config{})
	if err == nil {
		t.Fatal("expected error when APIKey is missing")
	}
}

func TestDialerLoadsFromStore(t *testing.T) {
	store := NewMemoryStore() // empty store

	_, err := Dialer(DirectConfig{CertStore: store})
	if err == nil {
		t.Fatal("expected error when store is empty")
	}
}

func TestDialer(t *testing.T) {
	cert := generateTestCert(t)

	d, err := Dialer(DirectConfig{
		Cert: cert,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.ingressEndpoint != defaultIngressEndpoint {
		t.Errorf("expected default ingress endpoint, got %s", d.ingressEndpoint)
	}
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func generateTestCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
}
