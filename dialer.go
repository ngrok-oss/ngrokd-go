package ngrokd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
)

// Dialer provides net.Dial-like access to ngrok bound endpoints
type Dialer struct {
	config         Config
	tlsConfig      *tls.Config
	operatorID     string
	apiClient      *apiClient
	logger         logr.Logger
	fallbackDialer ContextDialer

	mu        sync.RWMutex
	endpoints map[string]Endpoint // hostname -> endpoint cache

	closed    atomic.Bool
	closeOnce sync.Once
	closeCh   chan struct{}
	wg        sync.WaitGroup
}

// NewDialer creates a new Dialer with the given configuration
func NewDialer(ctx context.Context, cfg Config) (*Dialer, error) {
	cfg.setDefaults()

	d := &Dialer{
		config:         cfg,
		endpoints:      make(map[string]Endpoint),
		logger:         cfg.Logger,
		fallbackDialer: cfg.FallbackDialer,
		closeCh:        make(chan struct{}),
	}

	// Setup API client if we have an API key
	if cfg.APIKey != "" {
		d.apiClient = newAPIClient(cfg.APIKey)
	}

	// Get or provision certificate
	var tlsCert tls.Certificate
	var err error

	if cfg.TLSCert.Certificate != nil {
		// Use provided certificate
		tlsCert = cfg.TLSCert
		d.operatorID = cfg.OperatorID
	} else if cfg.APIKey != "" {
		// Auto-provision certificate using CertStore
		provisioner := newCertProvisioner(cfg.CertStore, d.apiClient, cfg.EndpointSelectors)
		tlsCert, d.operatorID, err = provisioner.EnsureCertificate(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to provision certificate: %w", err)
		}
		if d.logger.Enabled() {
			d.logger.Info("Certificate provisioned", "operatorID", d.operatorID)
		}
	} else {
		return nil, fmt.Errorf("either TLSCert or APIKey must be provided")
	}

	// Setup TLS config
	rootCAs := cfg.RootCAs
	if rootCAs == nil {
		rootCAs, _ = x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
	}

	d.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      rootCAs,
		// Enable session resumption for performance
		ClientSessionCache: tls.NewLRUClientSessionCache(128),
	}

	// Start background refresh if configured
	if cfg.RefreshInterval > 0 {
		d.wg.Add(1)
		go d.refreshLoop()
	}

	return d, nil
}

// refreshLoop runs background endpoint discovery at RefreshInterval
func (d *Dialer) refreshLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.closeCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if _, err := d.DiscoverEndpoints(ctx); err != nil {
				if d.logger.Enabled() {
					d.logger.Error(err, "Background endpoint refresh failed")
				}
			}
			cancel()
		}
	}
}

// Dial connects to the address via ngrok bound endpoint
// Address can be: hostname, hostname:port, or URL (https://hostname)
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext connects to the address via ngrok bound endpoint with context
// If the endpoint is not a known ngrok endpoint and FallbackDialer is set,
// the connection is routed through the fallback dialer instead.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.closed.Load() {
		return nil, ErrClosed
	}

	hostname, port, err := parseAddress(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}

	// Check if this is a known ngrok endpoint
	if !d.isKnownEndpoint(hostname) {
		if d.fallbackDialer != nil {
			if d.logger.Enabled() {
				d.logger.V(1).Info("Using fallback dialer", "hostname", hostname)
			}
			return d.fallbackDialer.DialContext(ctx, network, address)
		}
		return nil, &EndpointNotFoundError{Hostname: hostname}
	}

	// Dial with retry if configured
	return d.dialWithRetry(ctx, hostname, port)
}

// dialWithRetry attempts to dial with exponential backoff
func (d *Dialer) dialWithRetry(ctx context.Context, hostname string, port int) (net.Conn, error) {
	cfg := d.config.RetryConfig
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := d.calculateBackoff(attempt, cfg)
			if d.logger.Enabled() {
				d.logger.V(1).Info("Retrying dial", "attempt", attempt, "backoff", backoff)
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-d.closeCh:
				return nil, ErrClosed
			case <-time.After(backoff):
			}
		}

		conn, err := d.dialOnce(ctx, hostname, port)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		// Don't retry context errors
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}

// calculateBackoff returns the backoff duration for the given attempt
func (d *Dialer) calculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	backoff := float64(cfg.InitialBackoff)
	for i := 1; i < attempt; i++ {
		backoff *= cfg.BackoffMultiplier
	}

	// Add jitter (±25%)
	jitter := (rand.Float64() - 0.5) * 0.5 * backoff
	backoff += jitter

	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	return time.Duration(backoff)
}

// dialOnce performs a single dial attempt
func (d *Dialer) dialOnce(ctx context.Context, hostname string, port int) (net.Conn, error) {
	if d.logger.Enabled() {
		d.logger.V(1).Info("Dialing via ngrok", "hostname", hostname, "port", port)
	}

	// Dial mTLS to ngrok ingress
	ingressHost, _, _ := net.SplitHostPort(d.config.IngressEndpoint)
	if ingressHost == "" {
		ingressHost = d.config.IngressEndpoint
	}

	tlsConfig := d.tlsConfig.Clone()
	tlsConfig.ServerName = ingressHost

	// Fallback to InsecureSkipVerify if no custom CAs
	// (ngrok uses intermediate CA not in system trust stores)
	if d.config.RootCAs == nil {
		tlsConfig.InsecureSkipVerify = true
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: d.config.DialTimeout,
		},
		Config: tlsConfig,
	}

	address := d.config.IngressEndpoint
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, &DialError{Address: address, Cause: err}
	}

	// Upgrade connection with binding protocol
	resp, err := upgradeToBinding(conn, hostname, port)
	if err != nil {
		conn.Close()
		return nil, &UpgradeError{Hostname: hostname, Port: port, Cause: err}
	}

	if resp.ErrorCode != "" || resp.ErrorMessage != "" {
		conn.Close()
		return nil, &UpgradeError{
			Hostname: hostname,
			Port:     port,
			Message:  fmt.Sprintf("[%s] %s", resp.ErrorCode, resp.ErrorMessage),
		}
	}

	if d.logger.Enabled() {
		d.logger.V(1).Info("Connection upgraded",
			"endpointID", resp.EndpointID,
			"proto", resp.Proto)
	}

	return conn, nil
}

// DiscoverEndpoints fetches and caches bound endpoints from ngrok API
func (d *Dialer) DiscoverEndpoints(ctx context.Context) ([]Endpoint, error) {
	if d.closed.Load() {
		return nil, ErrClosed
	}

	endpoints, err := d.discoverEndpoints(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	d.mu.Lock()
	d.endpoints = make(map[string]Endpoint, len(endpoints))
	for _, ep := range endpoints {
		d.endpoints[ep.Hostname] = ep
	}
	d.mu.Unlock()

	return endpoints, nil
}

// Endpoints returns the cached endpoints
func (d *Dialer) Endpoints() map[string]Endpoint {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]Endpoint, len(d.endpoints))
	for k, v := range d.endpoints {
		result[k] = v
	}
	return result
}

// OperatorID returns the ngrok operator ID
func (d *Dialer) OperatorID() string {
	return d.operatorID
}

// isKnownEndpoint checks if the hostname matches a cached ngrok endpoint
func (d *Dialer) isKnownEndpoint(hostname string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.endpoints[hostname]
	return exists
}

// Close stops background goroutines and cleans up resources
func (d *Dialer) Close() error {
	d.closeOnce.Do(func() {
		d.closed.Store(true)
		close(d.closeCh)
	})
	d.wg.Wait()
	return nil
}
