package ngrokd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

func defaultCertDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ngrokd-go", "certs")
}

type certProvisioner struct {
	store     CertStore
	apiClient *apiClient
}

func newCertProvisioner(store CertStore, apiClient *apiClient) *certProvisioner {
	return &certProvisioner{
		store:     store,
		apiClient: apiClient,
	}
}

func (p *certProvisioner) EnsureCertificate(ctx context.Context) (cert tls.Certificate, operatorID string, err error) {
	// Check if certificate exists in store
	exists, err := p.store.Exists(ctx)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("failed to check store: %w", err)
	}

	if exists {
		keyPEM, certPEM, opID, err := p.store.Load(ctx)
		if err == nil {
			cert, err = tls.X509KeyPair(certPEM, keyPEM)
			if err == nil {
				return cert, opID, nil
			}
		}
		// Fall through to provision if load failed
	}

	// Provision new certificate
	return p.provisionCertificate(ctx)
}

func (p *certProvisioner) provisionCertificate(ctx context.Context) (tls.Certificate, string, error) {
	// Generate ECDSA P-384 private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("failed to generate key: %w", err)
	}

	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Create CSR
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{"ngrokd-sdk"},
		},
		SignatureAlgorithm: x509.ECDSAWithSHA384,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		return tls.Certificate{}, "", err
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})

	// Register with ngrok API
	// Use endpoint_selectors: ["true"] to match all kubernetes-bound endpoints
	operator, err := p.apiClient.CreateOperator(ctx, &operatorCreateRequest{
		Description:     "ngrokd-sdk",
		Metadata:        `{"type":"sdk"}`,
		EnabledFeatures: []string{"bindings"},
		Region:          "global",
		Binding: &operatorBindingCreate{
			EndpointSelectors: []string{"true"},
			CSR:               string(csrPEM),
		},
	})
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("failed to register: %w", err)
	}

	if operator.Binding == nil || operator.Binding.Cert.Cert == "" {
		return tls.Certificate{}, "", fmt.Errorf("no certificate in response")
	}

	certPEM := []byte(operator.Binding.Cert.Cert)

	// Save to store
	if err := p.store.Save(ctx, privateKeyPEM, certPEM, operator.ID); err != nil {
		return tls.Certificate{}, "", fmt.Errorf("failed to save certificate: %w", err)
	}

	cert, err := tls.X509KeyPair(certPEM, privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, "", err
	}

	return cert, operator.ID, nil
}
