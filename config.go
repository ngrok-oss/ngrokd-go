package ngrokd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
)

// Config holds the configuration for the ngrokd Dialer
type Config struct {
	// APIKey is the ngrok API key for provisioning certificates
	// Required if TLSCert is not provided
	APIKey string

	// OperatorID is the Kubernetes operator ID
	// If empty and APIKey is set, a new operator will be provisioned
	OperatorID string

	// TLSCert is the mTLS client certificate for connecting to ngrok
	// If empty, will be auto-provisioned using APIKey
	TLSCert tls.Certificate

	// CertStore is the storage backend for certificates.
	// Default: FileStore at ~/.ngrokd-go/certs
	CertStore CertStore

	// IngressEndpoint is the ngrok ingress endpoint
	// Default: kubernetes-binding-ingress.ngrok.io:443
	IngressEndpoint string

	// RootCAs is the CA pool for verifying ngrok ingress TLS
	// If nil, system roots are used (with fallback to InsecureSkipVerify)
	RootCAs *x509.CertPool

	// IngressDialer dials the ngrok ingress endpoint.
	// If nil, uses net.Dialer with 30s timeout.
	IngressDialer ContextDialer

	// Logger for structured logging
	Logger logr.Logger

	// DefaultDialer is used for addresses not matching known ngrok endpoints
	// If nil, DialContext returns an error for unknown endpoints
	DefaultDialer ContextDialer

	// EndpointSelectors are CEL expressions that filter which endpoints this operator can access.
	// Default: ["true"] (matches all endpoints)
	EndpointSelectors []string

	// PollingInterval is how often to poll the ngrok API for endpoints.
	// A background goroutine periodically calls DiscoverEndpoints.
	// Default: 30 seconds
	PollingInterval time.Duration
}

// ContextDialer matches the net.Dialer.DialContext signature
type ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func (c *Config) setDefaults() {
	// Read API key from environment if not set
	if c.APIKey == "" {
		c.APIKey = os.Getenv("NGROK_API_KEY")
	}
	if c.CertStore == nil {
		c.CertStore = NewFileStore("")
	}
	if c.IngressEndpoint == "" {
		c.IngressEndpoint = "kubernetes-binding-ingress.ngrok.io:443"
	}
	if c.IngressDialer == nil {
		c.IngressDialer = &net.Dialer{Timeout: 30 * time.Second}
	}
	if c.PollingInterval == 0 {
		c.PollingInterval = 30 * time.Second
	}
	if len(c.EndpointSelectors) == 0 {
		c.EndpointSelectors = []string{"true"}
	}
	if c.DefaultDialer == nil {
		c.DefaultDialer = &net.Dialer{}
	}
}
