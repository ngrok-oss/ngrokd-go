package ngrokd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
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
	// Default: FileStore at ~/.ngrokd-sdk/certs
	// Use MemoryStore for ephemeral environments, or implement
	// CertStore for AWS Secrets Manager, Vault, etc.
	CertStore CertStore

	// CertDir is the directory for FileStore (deprecated, use CertStore).
	// If CertStore is nil and CertDir is set, FileStore is used.
	CertDir string

	// IngressEndpoint is the ngrok ingress endpoint
	// Default: kubernetes-binding-ingress.ngrok.io:443
	IngressEndpoint string

	// RootCAs is the CA pool for verifying ngrok ingress TLS
	// If nil, system roots are used (with fallback to InsecureSkipVerify)
	RootCAs *x509.CertPool

	// DialTimeout is the timeout for establishing connections
	// Default: 30s
	DialTimeout time.Duration

	// Logger for structured logging
	Logger logr.Logger

	// FallbackDialer is used for addresses not matching known ngrok endpoints
	// If nil, DialContext returns an error for unknown endpoints
	FallbackDialer ContextDialer

	// EndpointSelectors are CEL expressions that filter which endpoints this operator can access.
	// Default: ["true"] (matches all endpoints)
	// Example: ["endpoint.metadata.name == 'my-service'"]
	EndpointSelectors []string

	// RefreshInterval is how often to poll the ngrok API for endpoints.
	// A background goroutine periodically calls DiscoverEndpoints.
	// Default: 30 seconds
	RefreshInterval time.Duration

	// RetryConfig configures retry behavior for transient failures
	RetryConfig RetryConfig
}

// RetryConfig configures exponential backoff retry behavior
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries)
	MaxRetries int

	// InitialBackoff is the initial backoff duration
	// Default: 100ms
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration
	// Default: 10s
	MaxBackoff time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	// Default: 2.0
	BackoffMultiplier float64
}

// ContextDialer matches the net.Dialer.DialContext signature
type ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func (c *Config) setDefaults() {
	if c.CertStore == nil {
		dir := c.CertDir
		if dir == "" {
			dir = defaultCertDir()
		}
		c.CertStore = NewFileStore(dir)
	}
	if c.IngressEndpoint == "" {
		c.IngressEndpoint = "kubernetes-binding-ingress.ngrok.io:443"
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 30 * time.Second
	}
	if c.RefreshInterval == 0 {
		c.RefreshInterval = 30 * time.Second
	}
	if len(c.EndpointSelectors) == 0 {
		c.EndpointSelectors = []string{"true"}
	}
	c.RetryConfig.setDefaults()
}

func (r *RetryConfig) setDefaults() {
	if r.InitialBackoff == 0 {
		r.InitialBackoff = 100 * time.Millisecond
	}
	if r.MaxBackoff == 0 {
		r.MaxBackoff = 10 * time.Second
	}
	if r.BackoffMultiplier == 0 {
		r.BackoffMultiplier = 2.0
	}
}
