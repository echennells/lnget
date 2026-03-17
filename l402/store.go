package l402

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/lntypes"
)

var (
	// ErrNoToken is the error returned when the store doesn't contain a
	// token for the requested domain.
	ErrNoToken = l402.ErrNoToken

	// ErrTokenExpired is the error returned when a token has expired.
	ErrTokenExpired = errors.New("token expired")

	// ErrInvalidDomain is returned when a domain sanitizes to an unsafe
	// value such as "..", ".", or an empty string.
	ErrInvalidDomain = errors.New("invalid domain")
)

// Store manages L402 tokens on a per-domain basis. Unlike the aperture Store
// which has a single current token, this store maps domains to tokens to
// support multiple services.
type Store interface {
	// GetToken retrieves the current valid token for a domain.
	// Returns ErrNoToken if no token exists for the domain.
	GetToken(domain string) (*Token, error)

	// StoreToken saves or updates a token for a domain.
	StoreToken(domain string, token *Token) error

	// AllTokens returns all stored tokens mapped by domain.
	AllTokens() (map[string]*Token, error)

	// RemoveToken deletes the token for a domain.
	RemoveToken(domain string) error

	// HasPendingPayment checks if there's a pending payment for a domain.
	HasPendingPayment(domain string) bool

	// StorePending stores a pending (unpaid) token for a domain.
	StorePending(domain string, token *Token) error

	// RemovePending removes a pending token for a domain.
	RemovePending(domain string) error
}

// DomainFromURL extracts the domain (host:port if non-standard) from a URL.
// This is used as the key for per-domain token storage.
func DomainFromURL(u *url.URL) string {
	host := u.Hostname()
	port := u.Port()

	// Include port for non-standard ports.
	if port != "" && port != "80" && port != "443" {
		return host + ":" + port
	}

	return host
}

// SanitizeDomain converts a domain to a filesystem-safe string. It returns
// an error if the result is empty or a path traversal component ("." or "..").
func SanitizeDomain(domain string) (string, error) {
	// Replace colons with underscores for filesystem compatibility.
	result := make([]byte, 0, len(domain))

	for i := 0; i < len(domain); i++ {
		c := domain[i]
		if c == ':' {
			result = append(result, '_')

			continue
		}

		// Check if character is alphanumeric or allowed punctuation.
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isAllowed := c == '.' || c == '-' || c == '_'

		if isLower || isUpper || isDigit || isAllowed {
			result = append(result, c)
		}
	}

	s := string(result)

	// Reject empty results and path traversal components.
	if s == "" || s == "." || s == ".." {
		return "", ErrInvalidDomain
	}

	return s, nil
}

// GetOriginalDomain attempts to reverse the sanitization to get the original
// domain. It first checks for a .domain metadata file (written by StoreToken),
// then falls back to a best-effort heuristic that replaces the last underscore
// with a colon (for host:port). The last underscore is used because port
// numbers never contain underscores, while hostnames may.
func GetOriginalDomain(baseDir, sanitized string) string {
	// Try reading the metadata file first.
	metaPath := filepath.Join(baseDir, sanitized, ".domain")
	if data, err := os.ReadFile(metaPath); err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			return s
		}
	}

	// Fallback: replace the last underscore with a colon, since port
	// numbers cannot contain underscores.
	idx := strings.LastIndex(sanitized, "_")
	if idx >= 0 {
		return sanitized[:idx] + ":" + sanitized[idx+1:]
	}

	return sanitized
}

// storeDomainMetadata writes the original domain name to a .domain file
// inside the sanitized directory, so AllTokens can recover it losslessly.
func storeDomainMetadata(baseDir, domain string) error {
	sanitized, err := SanitizeDomain(domain)
	if err != nil {
		return err
	}

	dir := filepath.Join(baseDir, sanitized)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(
		filepath.Join(dir, ".domain"),
		[]byte(domain+"\n"), 0600,
	)
}

// zeroPreimage is an empty preimage used to check pending status.
var zeroPreimage lntypes.Preimage

// IsPending returns true if the token payment is still pending (no preimage).
func IsPending(token *Token) bool {
	if token == nil {
		return true
	}

	return token.Preimage == zeroPreimage
}
