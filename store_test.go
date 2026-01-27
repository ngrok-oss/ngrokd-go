package ngrokd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Initially empty
	exists, err := store.Exists(ctx)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected empty store")
	}

	// Save
	key := []byte("private-key-data")
	cert := []byte("certificate-data")
	opID := "op_123"

	if err := store.Save(ctx, key, cert, opID); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Now exists
	exists, _ = store.Exists(ctx)
	if !exists {
		t.Error("expected store to have data")
	}

	// Load
	loadedKey, loadedCert, loadedOpID, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if string(loadedKey) != string(key) {
		t.Errorf("key mismatch: got %s, want %s", loadedKey, key)
	}
	if string(loadedCert) != string(cert) {
		t.Errorf("cert mismatch: got %s, want %s", loadedCert, cert)
	}
	if loadedOpID != opID {
		t.Errorf("operatorID mismatch: got %s, want %s", loadedOpID, opID)
	}
}

func TestMemoryStoreWithCert(t *testing.T) {
	ctx := context.Background()

	key := []byte("preloaded-key")
	cert := []byte("preloaded-cert")
	opID := "op_preloaded"

	store := NewMemoryStoreWithCert(key, cert, opID)

	exists, _ := store.Exists(ctx)
	if !exists {
		t.Error("expected preloaded store to exist")
	}

	loadedKey, loadedCert, loadedOpID, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if string(loadedKey) != string(key) {
		t.Errorf("key mismatch")
	}
	if string(loadedCert) != string(cert) {
		t.Errorf("cert mismatch")
	}
	if loadedOpID != opID {
		t.Errorf("operatorID mismatch")
	}
}

func TestFileStore(t *testing.T) {
	ctx := context.Background()

	// Use temp directory
	tmpDir := filepath.Join(os.TempDir(), "ngrokd-test-store")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)

	// Initially empty
	exists, err := store.Exists(ctx)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected empty store")
	}

	// Save
	key := []byte("file-private-key")
	cert := []byte("file-certificate")
	opID := "op_file_123"

	if err := store.Save(ctx, key, cert, opID); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Now exists
	exists, _ = store.Exists(ctx)
	if !exists {
		t.Error("expected store to have data")
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(tmpDir, "tls.key")); err != nil {
		t.Errorf("tls.key not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "tls.crt")); err != nil {
		t.Errorf("tls.crt not created: %v", err)
	}

	// Load
	loadedKey, loadedCert, loadedOpID, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if string(loadedKey) != string(key) {
		t.Errorf("key mismatch")
	}
	if string(loadedCert) != string(cert) {
		t.Errorf("cert mismatch")
	}
	if loadedOpID != opID {
		t.Errorf("operatorID mismatch")
	}
}
