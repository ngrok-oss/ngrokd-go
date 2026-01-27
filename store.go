package ngrokd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CertStore abstracts certificate storage for the SDK.
// Implement this interface to store certificates in:
// - AWS Secrets Manager
// - HashiCorp Vault
// - GCP Secret Manager
// - Database
// - etc.
type CertStore interface {
	// Load retrieves the stored certificate, private key, and operator ID.
	// Returns error if not found or storage fails.
	Load(ctx context.Context) (key, cert []byte, operatorID string, err error)

	// Save stores the certificate, private key, and operator ID.
	Save(ctx context.Context, key, cert []byte, operatorID string) error

	// Exists checks if a certificate is already stored.
	Exists(ctx context.Context) (bool, error)
}

// FileStore stores certificates on the local filesystem.
// This is the default storage backend.
type FileStore struct {
	// Dir is the directory to store certificates.
	// Files created: tls.key, tls.crt, operator_id
	Dir string
}

// NewFileStore creates a FileStore with the given directory.
func NewFileStore(dir string) *FileStore {
	if dir == "" {
		dir = defaultCertDir()
	}
	return &FileStore{Dir: dir}
}

func (s *FileStore) keyPath() string      { return filepath.Join(s.Dir, "tls.key") }
func (s *FileStore) certPath() string     { return filepath.Join(s.Dir, "tls.crt") }
func (s *FileStore) operatorPath() string { return filepath.Join(s.Dir, "operator_id") }

func (s *FileStore) Exists(ctx context.Context) (bool, error) {
	_, keyErr := os.Stat(s.keyPath())
	_, certErr := os.Stat(s.certPath())
	return keyErr == nil && certErr == nil, nil
}

func (s *FileStore) Load(ctx context.Context) (key, cert []byte, operatorID string, err error) {
	key, err = os.ReadFile(s.keyPath())
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read key: %w", err)
	}

	cert, err = os.ReadFile(s.certPath())
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read cert: %w", err)
	}

	// Operator ID is optional (for backwards compat)
	opData, _ := os.ReadFile(s.operatorPath())
	operatorID = string(opData)

	return key, cert, operatorID, nil
}

func (s *FileStore) Save(ctx context.Context, key, cert []byte, operatorID string) error {
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(s.keyPath(), key, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	if err := os.WriteFile(s.certPath(), cert, 0644); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}

	if err := os.WriteFile(s.operatorPath(), []byte(operatorID), 0644); err != nil {
		return fmt.Errorf("failed to write operator ID: %w", err)
	}

	return nil
}

// MemoryStore stores certificates in memory only.
// Certificates are lost when the process exits.
// Useful for ephemeral environments or testing.
type MemoryStore struct {
	mu         sync.RWMutex
	key        []byte
	cert       []byte
	operatorID string
	stored     bool
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// NewMemoryStoreWithCert creates a MemoryStore pre-loaded with a certificate.
// Useful when you have the cert from an external source (e.g., Secrets Manager).
func NewMemoryStoreWithCert(key, cert []byte, operatorID string) *MemoryStore {
	return &MemoryStore{
		key:        key,
		cert:       cert,
		operatorID: operatorID,
		stored:     true,
	}
}

func (s *MemoryStore) Exists(ctx context.Context) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stored, nil
}

func (s *MemoryStore) Load(ctx context.Context) (key, cert []byte, operatorID string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.stored {
		return nil, nil, "", fmt.Errorf("no certificate stored")
	}

	return s.key, s.cert, s.operatorID, nil
}

func (s *MemoryStore) Save(ctx context.Context, key, cert []byte, operatorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.key = make([]byte, len(key))
	copy(s.key, key)

	s.cert = make([]byte, len(cert))
	copy(s.cert, cert)

	s.operatorID = operatorID
	s.stored = true

	return nil
}
