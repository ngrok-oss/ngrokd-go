package ngrokd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"

	"github.com/go-logr/logr"
)

const defaultIngressEndpoint = "kubernetes-binding-ingress.ngrok.io:443"

// Config holds the configuration for a Dialer with API-based discovery.
type Config struct {
	// APIKey is the ngrok API key for provisioning certificates and discovering endpoints.
	// Required.
	APIKey string

	// OperatorID is an existing operator ID to use for discovery.
	// If empty, will be loaded from CertStore or provisioned.
	OperatorID string

	// Cert is an existing mTLS certificate to use.
	// If empty, will be loaded from CertStore or provisioned.
	Cert tls.Certificate

	// CertStore is the storage backend for certificates.
	// Default: FileStore at ~/.ngrokd-go/certs
	CertStore CertStore

	// IngressEndpoint is the ngrok ingress endpoint.
	// Default: kubernetes-binding-ingress.ngrok.io:443
	IngressEndpoint string

	// RootCAs is the CA pool for verifying ngrok ingress TLS.
	// If nil, system roots are used (with fallback to InsecureSkipVerify).
	RootCAs *x509.CertPool

	// IngressDialer dials the ngrok ingress endpoint.
	// If nil, uses net.Dialer with 30s timeout.
	IngressDialer ContextDialer

	// Logger for structured logging.
	Logger logr.Logger

	// EndpointSelectors are CEL expressions that filter which endpoints this operator can access.
	// Default: ["true"] (matches all endpoints)
	EndpointSelectors []string
}

// DirectConfig holds the configuration for a Dialer without API access.
type DirectConfig struct {
	// Cert is the mTLS client certificate for connecting to ngrok.
	// If empty, loads from CertStore.
	Cert tls.Certificate

	// CertStore is the storage backend to load certificates from.
	// Only used if Cert is not provided.
	// Default: FileStore at ~/.ngrokd-go/certs
	CertStore CertStore

	// IngressEndpoint is the ngrok ingress endpoint.
	// Default: kubernetes-binding-ingress.ngrok.io:443
	IngressEndpoint string

	// RootCAs is the CA pool for verifying ngrok ingress TLS.
	// If nil, system roots are used (with fallback to InsecureSkipVerify).
	RootCAs *x509.CertPool

	// IngressDialer dials the ngrok ingress endpoint.
	// If nil, uses net.Dialer with 30s timeout.
	IngressDialer ContextDialer

	// Logger for structured logging.
	Logger logr.Logger
}

// ContextDialer matches the net.Dialer.DialContext signature.
type ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func (c *Config) setDefaults() {
	if c.CertStore == nil {
		c.CertStore = NewFileStore("")
	}
	if c.IngressEndpoint == "" {
		c.IngressEndpoint = defaultIngressEndpoint
	}
	if c.IngressDialer == nil {
		c.IngressDialer = defaultDialer()
	}
	if len(c.EndpointSelectors) == 0 {
		c.EndpointSelectors = []string{"true"}
	}
}

func (c *DirectConfig) setDefaults() {
	if c.CertStore == nil {
		c.CertStore = NewFileStore("")
	}
	if c.IngressEndpoint == "" {
		c.IngressEndpoint = defaultIngressEndpoint
	}
	if c.IngressDialer == nil {
		c.IngressDialer = defaultDialer()
	}
}

func defaultDialer() ContextDialer {
	return &net.Dialer{Timeout: 30 * 1e9} // 30 seconds
}
