package l402

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewFileStore tests FileStore creation.
func TestNewFileStore(t *testing.T) {
	// Create a temp directory.
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "tokens")

	// Create FileStore.
	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Verify directory was created.
	_, err = os.Stat(storePath)
	if os.IsNotExist(err) {
		t.Errorf("store directory was not created")
	}

	// Verify store is not nil.
	if store == nil {
		t.Errorf("store is nil")
	}
}

// TestFileStoreGetTokenNotFound tests getting a non-existent token.
func TestFileStoreGetTokenNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Try to get a token that doesn't exist.
	_, err = store.GetToken("nonexistent.com")
	if err == nil {
		t.Errorf("expected error for non-existent token")
	}
}

// TestFileStoreAllTokensEmpty tests getting all tokens from empty store.
func TestFileStoreAllTokensEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	tokens, err := store.AllTokens()
	if err != nil {
		t.Fatalf("AllTokens() error = %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("AllTokens() returned %d tokens, want 0", len(tokens))
	}
}

// TestFileStoreListDomainsEmpty tests listing domains from empty store.
func TestFileStoreListDomainsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	domains, err := store.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains() error = %v", err)
	}

	if len(domains) != 0 {
		t.Errorf("ListDomains() returned %d domains, want 0",
			len(domains))
	}
}

// TestFileStoreHasPendingPayment tests pending payment detection.
func TestFileStoreHasPendingPayment(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Should return false for non-existent domain.
	if store.HasPendingPayment("nonexistent.com") {
		t.Errorf("HasPendingPayment() = true for non-existent domain")
	}
}

// TestFileStoreRemoveToken tests removing a non-existent token.
func TestFileStoreRemoveToken(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Removing a non-existent token should not error.
	err = store.RemoveToken("nonexistent.com")
	if err != nil {
		t.Errorf("RemoveToken() error = %v, want nil", err)
	}
}

// TestFileStoreDomainDir tests the domain directory path.
func TestFileStoreDomainDir(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Test domain dir for a simple domain.
	expected := filepath.Join(tmpDir, "example.com")

	result, err := store.domainDir("example.com")
	if err != nil {
		t.Fatalf("domainDir() error = %v", err)
	}

	if result != expected {
		t.Errorf("domainDir() = %q, want %q", result, expected)
	}

	// Test domain dir with port.
	expected = filepath.Join(tmpDir, "example.com_8080")

	result, err = store.domainDir("example.com:8080")
	if err != nil {
		t.Fatalf("domainDir() error = %v", err)
	}

	if result != expected {
		t.Errorf("domainDir() = %q, want %q", result, expected)
	}
}

// TestFileStoreGetDomainStore tests getting a domain-specific store.
func TestFileStoreGetDomainStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Getting a domain store should create the domain directory.
	domain := "test.example.com"

	_, err = store.getDomainStore(domain)
	if err != nil {
		t.Fatalf("getDomainStore() error = %v", err)
	}

	// Verify domain directory was created.
	domainDir, err := store.domainDir(domain)
	if err != nil {
		t.Fatalf("domainDir() error = %v", err)
	}

	_, err = os.Stat(domainDir)
	if os.IsNotExist(err) {
		t.Errorf("domain directory was not created")
	}
}

// TestFileStoreAllTokensWithNonDirEntries tests AllTokens with files in dir.
func TestFileStoreAllTokensWithNonDirEntries(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Create a regular file in the store directory (not a domain dir).
	filePath := filepath.Join(tmpDir, "not-a-domain.txt")

	err = os.WriteFile(filePath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// AllTokens should skip the file and not error.
	tokens, err := store.AllTokens()
	if err != nil {
		t.Fatalf("AllTokens() error = %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("AllTokens() returned %d tokens, want 0", len(tokens))
	}
}

// TestFileStoreListDomainsWithNonDirEntries tests ListDomains with files.
func TestFileStoreListDomainsWithNonDirEntries(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Create a regular file in the store directory.
	filePath := filepath.Join(tmpDir, "not-a-domain.txt")

	err = os.WriteFile(filePath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// ListDomains should skip the file and not error.
	domains, err := store.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains() error = %v", err)
	}

	if len(domains) != 0 {
		t.Errorf("ListDomains() returned %d domains, want 0",
			len(domains))
	}
}

// TestFileStoreRemoveTokenPathTraversal verifies that RemoveToken rejects
// domains that would resolve outside the base directory.
func TestFileStoreRemoveTokenPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// SanitizeDomain strips slashes, so "../../etc" becomes "....etc"
	// which is safe. But ".." sanitizes to ".." which is explicitly
	// rejected by SanitizeDomain.
	err = store.RemoveToken("..")
	if err == nil {
		t.Errorf("RemoveToken(..) should have returned error")
	}

	// Single dot should also be rejected.
	err = store.RemoveToken(".")
	if err == nil {
		t.Errorf("RemoveToken(.) should have returned error")
	}

	// Empty domain should be rejected.
	err = store.RemoveToken("")
	if err == nil {
		t.Errorf("RemoveToken('') should have returned error")
	}

	// Verify that a path with slashes is sanitized to a safe value
	// (slashes are stripped, so the result stays in baseDir).
	err = store.RemoveToken("../../etc")
	if err != nil {
		t.Errorf("RemoveToken(../../etc) unexpectedly errored: %v "+
			"(slashes are stripped by sanitizer)", err)
	}
}

// TestFileStoreRemovePendingNonExistent tests removing non-existent pending.
func TestFileStoreRemovePendingNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// RemovePending on non-existent domain should not panic.
	// It may error or succeed depending on aperture implementation.
	_ = store.RemovePending("nonexistent.com")
}
