package l402

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lightninglabs/aperture/l402"
)

// FileStore is an implementation of the Store interface that uses files to
// save the serialized tokens. Tokens are stored per-domain in separate
// directories, with each domain directory containing an aperture FileStore.
type FileStore struct {
	// baseDir is the base directory where all domain directories are
	// stored.
	baseDir string
}

// A compile-time flag to ensure that FileStore implements the Store interface.
var _ Store = (*FileStore)(nil)

// NewFileStore creates a new file based token store, creating its directory
// structure in the provided base directory.
func NewFileStore(baseDir string) (*FileStore, error) {
	// Create the base directory if it doesn't exist.
	err := os.MkdirAll(baseDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create token store dir: %w",
			err)
	}

	return &FileStore{
		baseDir: baseDir,
	}, nil
}

// domainDir returns the directory path for a domain's tokens. It returns
// an error if the domain sanitizes to an unsafe value.
func (f *FileStore) domainDir(domain string) (string, error) {
	sanitized, err := SanitizeDomain(domain)
	if err != nil {
		return "", err
	}

	return filepath.Join(f.baseDir, sanitized), nil
}

// getDomainStore returns the aperture FileStore for a specific domain.
func (f *FileStore) getDomainStore(domain string) (*l402.FileStore, error) {
	dir, err := f.domainDir(domain)
	if err != nil {
		return nil, err
	}

	return l402.NewFileStore(dir)
}

// GetToken retrieves the current valid token for a domain.
func (f *FileStore) GetToken(domain string) (*Token, error) {
	store, err := f.getDomainStore(domain)
	if err != nil {
		return nil, err
	}

	return store.CurrentToken()
}

// StoreToken saves or updates a token for a domain. It also writes a
// .domain metadata file so that AllTokens can recover the original
// domain name losslessly (SanitizeDomain is lossy for underscored
// domains).
func (f *FileStore) StoreToken(domain string, token *Token) error {
	store, err := f.getDomainStore(domain)
	if err != nil {
		return err
	}

	if err := store.StoreToken(token); err != nil {
		return err
	}

	// Best-effort: write domain metadata for lossless round-trip.
	_ = storeDomainMetadata(f.baseDir, domain)

	return nil
}

// AllTokens returns all stored tokens mapped by domain.
func (f *FileStore) AllTokens() (map[string]*Token, error) {
	tokens := make(map[string]*Token)

	// List all domain directories.
	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return tokens, nil
		}

		return nil, fmt.Errorf("failed to read token store: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// The directory name is the sanitized domain.
		sanitizedDomain := entry.Name()

		// Try to read the token.
		token, err := f.GetToken(sanitizedDomain)
		if err != nil {
			if errors.Is(err, ErrNoToken) {
				continue
			}

			return nil, err
		}

		// Use the original domain as the key.
		originalDomain := GetOriginalDomain(f.baseDir, sanitizedDomain)
		tokens[originalDomain] = token
	}

	return tokens, nil
}

// RemoveToken deletes the token for a domain. It validates that the
// resolved path stays within the base directory to prevent path
// traversal attacks.
func (f *FileStore) RemoveToken(domain string) error {
	domainDir, err := f.domainDir(domain)
	if err != nil {
		return err
	}

	// Resolve to absolute path and verify it stays within baseDir.
	absDir, err := filepath.Abs(domainDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	absBase, err := filepath.Abs(f.baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}

	// The resolved path must be a child of the base directory.
	if !strings.HasPrefix(absDir, absBase+string(filepath.Separator)) {
		return fmt.Errorf("path escapes token store: %w",
			ErrInvalidDomain)
	}

	// Remove the entire domain directory.
	err = os.RemoveAll(domainDir)
	if err != nil {
		return fmt.Errorf("failed to remove token: %w", err)
	}

	return nil
}

// HasPendingPayment checks if there's a pending payment for a domain.
func (f *FileStore) HasPendingPayment(domain string) bool {
	token, err := f.GetToken(domain)
	if err != nil {
		return false
	}

	return IsPending(token)
}

// StorePending stores a pending (unpaid) token for a domain.
func (f *FileStore) StorePending(domain string, token *Token) error {
	// The aperture FileStore handles pending tokens automatically.
	return f.StoreToken(domain, token)
}

// RemovePending removes a pending token for a domain.
func (f *FileStore) RemovePending(domain string) error {
	store, err := f.getDomainStore(domain)
	if err != nil {
		return err
	}

	return store.RemovePendingToken()
}

// ListDomains returns all domains that have stored tokens.
func (f *FileStore) ListDomains() ([]string, error) {
	var domains []string

	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return domains, nil
		}

		return nil, fmt.Errorf("failed to read token store: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if there's actually a token in this directory.
		sanitizedDomain := entry.Name()

		_, err := f.GetToken(sanitizedDomain)
		if err == nil {
			domains = append(domains, GetOriginalDomain(f.baseDir, sanitizedDomain))
		}
	}

	return domains, nil
}
