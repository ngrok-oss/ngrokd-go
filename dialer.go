package ngrokd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/go-logr/logr"
)

// dialer provides simple net.Dial-like access to ngrok endpoints.
type dialer struct {
	tlsConfig       *tls.Config
	ingressEndpoint string
	ingressDialer   ContextDialer
	rootCAs         *x509.CertPool
	logger          logr.Logger
}

// Dialer creates a dialer for direct connections to ngrok endpoints.
// If no Cert is provided, loads from CertStore (default: ~/.ngrokd-go/certs).
func Dialer(cfg DirectConfig) (*dialer, error) {
	cfg.setDefaults()

	var cert tls.Certificate
	if cfg.Cert.Certificate != nil {
		cert = cfg.Cert
	} else {
		// Load from store
		ctx := context.Background()
		exists, err := cfg.CertStore.Exists(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check cert store: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("no certificate found; provision with DiscoveryDialer first or provide Cert")
		}

		keyPEM, certPEM, _, err := cfg.CertStore.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}

		cert, err = tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
	}

	return &dialer{
		tlsConfig:       buildTLSConfig(cert, cfg.RootCAs),
		ingressEndpoint: cfg.IngressEndpoint,
		ingressDialer:   cfg.IngressDialer,
		rootCAs:         cfg.RootCAs,
		logger:          cfg.Logger,
	}, nil
}

// Dial connects to the address via ngrok.
func (d *dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext connects to the address via ngrok with context.
func (d *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	hostname, port, err := parseAddress(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}

	if d.logger.Enabled() {
		d.logger.V(1).Info("Dialing via ngrok", "hostname", hostname, "port", port)
	}

	return dialNgrok(ctx, d.ingressDialer, d.ingressEndpoint, d.tlsConfig, d.rootCAs, hostname, port, d.logger)
}

// discoveryDialer provides net.Dial-like access with API-based cert provisioning and visibility.
type discoveryDialer struct {
	tlsConfig       *tls.Config
	ingressEndpoint string
	ingressDialer   ContextDialer
	rootCAs         *x509.CertPool
	logger          logr.Logger
	operatorID      string
	apiClient       *apiClient
}

// DiscoveryDialer creates a dialer with API-based cert provisioning and endpoint visibility.
// Requires an API key for provisioning certificates. Use Endpoints() or Diagnose() to see available endpoints.
func DiscoveryDialer(ctx context.Context, cfg Config) (*discoveryDialer, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("APIKey is required; use Dialer for direct connections")
	}

	cfg.setDefaults()

	apiClient := newAPIClient(cfg.APIKey)

	// Use provided cert/operator, or provision/load from store
	var tlsCert tls.Certificate
	var operatorID string

	if cfg.Cert.Certificate != nil {
		tlsCert = cfg.Cert
		operatorID = cfg.OperatorID
	} else {
		provisioner := newCertProvisioner(cfg.CertStore, apiClient, cfg.EndpointSelectors)
		var err error
		tlsCert, operatorID, err = provisioner.EnsureCertificate(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to provision certificate: %w", err)
		}
	}

	// Allow overriding operator ID even with provisioned cert
	if cfg.OperatorID != "" {
		operatorID = cfg.OperatorID
	}

	d := &discoveryDialer{
		tlsConfig:       buildTLSConfig(tlsCert, cfg.RootCAs),
		ingressEndpoint: cfg.IngressEndpoint,
		ingressDialer:   cfg.IngressDialer,
		rootCAs:         cfg.RootCAs,
		logger:          cfg.Logger,
		operatorID:      operatorID,
		apiClient:       apiClient,
	}

	if d.logger.Enabled() {
		d.logger.Info("Certificate ready", "operatorID", d.operatorID)
	}

	return d, nil
}

// Dial connects to the address via ngrok.
func (d *discoveryDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext connects to the address via ngrok with context.
func (d *discoveryDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	hostname, port, err := parseAddress(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}

	if d.logger.Enabled() {
		d.logger.V(1).Info("Dialing via ngrok", "hostname", hostname, "port", port)
	}

	return dialNgrok(ctx, d.ingressDialer, d.ingressEndpoint, d.tlsConfig, d.rootCAs, hostname, port, d.logger)
}

// OperatorID returns the ngrok operator ID.
func (d *discoveryDialer) OperatorID() string {
	return d.operatorID
}

// Endpoints fetches bound endpoints from ngrok API.
func (d *discoveryDialer) Endpoints(ctx context.Context) ([]Endpoint, error) {
	return discoverEndpoints(ctx, d.apiClient, d.operatorID)
}



// dialNgrok is the shared dial implementation.
func dialNgrok(ctx context.Context, ingressDialer ContextDialer, ingressEndpoint string, tlsConfig *tls.Config, rootCAs *x509.CertPool, hostname string, port int, logger logr.Logger) (net.Conn, error) {
	ingressHost, _, _ := net.SplitHostPort(ingressEndpoint)
	if ingressHost == "" {
		ingressHost = ingressEndpoint
	}

	tlsCfg := tlsConfig.Clone()
	tlsCfg.ServerName = ingressHost

	if rootCAs == nil {
		tlsCfg.InsecureSkipVerify = true
	}

	tcpConn, err := ingressDialer.DialContext(ctx, "tcp", ingressEndpoint)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", ingressEndpoint, err)
	}

	tlsConn := tls.Client(tcpConn, tlsCfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("TLS handshake %s: %w", ingressEndpoint, err)
	}

	endpointID, proto, err := upgradeToBinding(tlsConn, hostname, port)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("upgrade %s:%d: %w", hostname, port, err)
	}

	if logger.Enabled() {
		logger.V(1).Info("Connection upgraded", "endpointID", endpointID, "proto", proto)
	}

	return tlsConn, nil
}

// buildTLSConfig creates a TLS config with the given certificate and CA pool.
func buildTLSConfig(cert tls.Certificate, rootCAs *x509.CertPool) *tls.Config {
	if rootCAs == nil {
		rootCAs, _ = x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            rootCAs,
		ClientSessionCache: tls.NewLRUClientSessionCache(128),
	}
}
