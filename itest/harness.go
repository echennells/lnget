//go:build itest
// +build itest

package itest

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

const (
	// defaultTimeout is the default timeout for test operations.
	defaultTimeout = 30 * time.Second

	// serverStartTimeout is the time allowed for the mock server to start.
	serverStartTimeout = 5 * time.Second
)

// Harness provides a test environment for lnget integration tests. It includes
// a mock L402 server and a mock Lightning Network backend.
type Harness struct {
	t *testing.T

	// mockServer is the mock HTTP server that returns L402 challenges.
	mockServer *MockServer

	// mockLN is the mock Lightning Network backend for paying invoices.
	mockLN *MockLNBackend

	// tokenStore is the token store used by the test client.
	tokenStore l402.Store

	// cfg is the configuration used for tests.
	cfg *config.Config

	// tempDir is a temporary directory for test artifacts.
	tempDir string
}

// NewHarness creates a new test harness with a mock server and mock LN backend.
func NewHarness(t *testing.T) *Harness {
	t.Helper()

	// Create a temporary directory for tokens and other artifacts.
	tempDir, err := os.MkdirTemp("", "lnget-itest-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create the token store.
	tokenStore, err := l402.NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create token store: %v", err)
	}

	// Create the mock LN backend.
	mockLN := NewMockLNBackend()

	// Create a default configuration.
	cfg := config.DefaultConfig()
	cfg.Tokens.Dir = tempDir

	h := &Harness{
		t:          t,
		mockLN:     mockLN,
		tokenStore: tokenStore,
		cfg:        cfg,
		tempDir:    tempDir,
	}

	return h
}

// Start starts the mock server and prepares the harness for testing.
func (h *Harness) Start(ctx context.Context) error {
	// Find an available port for the mock server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	// Close the listener so the server can use the port.
	if err := listener.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	// Create and start the mock server.
	h.mockServer = NewMockServer(port, h.mockLN)

	if err := h.mockServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start mock server: %w", err)
	}

	// Wait for server to be ready.
	if err := h.waitForServer(ctx, serverStartTimeout); err != nil {
		return fmt.Errorf("mock server failed to start: %w", err)
	}

	return nil
}

// Stop stops the mock server and cleans up resources.
func (h *Harness) Stop() {
	if h.mockServer != nil {
		_ = h.mockServer.Stop()
	}

	// Clean up temporary directory.
	if h.tempDir != "" {
		_ = os.RemoveAll(h.tempDir)
	}
}

// ServerURL returns the base URL of the mock server.
func (h *Harness) ServerURL() string {
	if h.mockServer == nil {
		return ""
	}

	return h.mockServer.URL()
}

// Config returns the test configuration.
func (h *Harness) Config() *config.Config {
	return h.cfg
}

// TokenStore returns the token store.
func (h *Harness) TokenStore() l402.Store {
	return h.tokenStore
}

// MockLN returns the mock Lightning Network backend.
func (h *Harness) MockLN() *MockLNBackend {
	return h.mockLN
}

// MockServer returns the mock HTTP server.
func (h *Harness) MockServer() *MockServer {
	return h.mockServer
}

// NewClient creates a new lnget client configured for testing.
func (h *Harness) NewClient() (*client.Client, error) {
	return client.NewClient(&client.ClientConfig{
		Config:  h.cfg,
		Store:   h.tokenStore,
		Backend: h.mockLN,
	})
}

// SetMaxCost sets the maximum cost for automatic payments.
func (h *Harness) SetMaxCost(sats int64) {
	h.cfg.L402.MaxCostSats = sats
}

// SetMaxFee sets the maximum routing fee.
func (h *Harness) SetMaxFee(sats int64) {
	h.cfg.L402.MaxFeeSats = sats
}

// waitForServer waits for the mock server to become available.
func (h *Harness) waitForServer(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", h.mockServer.port)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to connect to the server.
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()

			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for server at %s", addr)
}

// MockLNBackend is a mock implementation of the ln.Backend interface for
// testing purposes.
type MockLNBackend struct {
	// PaymentResults maps payment hashes to (preimage, error) pairs.
	PaymentResults map[string]paymentResult

	// Payments tracks all payments made.
	Payments []Payment

	// DefaultPreimage is returned when no specific result is configured.
	DefaultPreimage lntypes.Preimage
}

// Payment records a payment request for testing.
type Payment struct {
	Invoice   string
	MaxFeeSat int64
	Timeout   time.Duration
}

// paymentResult holds the result of a simulated payment.
type paymentResult struct {
	Preimage lntypes.Preimage
	Err      error
}

// NewMockLNBackend creates a new mock LN backend with default behavior.
func NewMockLNBackend() *MockLNBackend {
	// Default preimage for testing.
	var defaultPreimage lntypes.Preimage
	for i := range defaultPreimage {
		defaultPreimage[i] = byte(i)
	}

	return &MockLNBackend{
		PaymentResults:  make(map[string]paymentResult),
		Payments:        make([]Payment, 0),
		DefaultPreimage: defaultPreimage,
	}
}

// PayInvoice implements the ln.Backend interface. It simulates paying an
// invoice and returns a preimage.
func (m *MockLNBackend) PayInvoice(ctx context.Context, invoice string,
	maxFeeSat int64, timeout time.Duration) (*l402.PaymentResult, error) {

	// Track the payment.
	m.Payments = append(m.Payments, Payment{
		Invoice:   invoice,
		MaxFeeSat: maxFeeSat,
		Timeout:   timeout,
	})

	// Check for configured result by invoice (for simplicity in tests).
	if result, ok := m.PaymentResults[invoice]; ok {
		if result.Err != nil {
			return nil, result.Err
		}

		return &l402.PaymentResult{
			Preimage:       result.Preimage,
			AmountPaid:     lnwire.MilliSatoshi(100000), // 100 sats.
			RoutingFeePaid: lnwire.MilliSatoshi(1000),   // 1 sat fee.
		}, nil
	}

	// Return default success.
	return &l402.PaymentResult{
		Preimage:       m.DefaultPreimage,
		AmountPaid:     lnwire.MilliSatoshi(100000),
		RoutingFeePaid: lnwire.MilliSatoshi(1000),
	}, nil
}

// GetInfo implements the ln.Backend interface.
func (m *MockLNBackend) GetInfo(ctx context.Context) (*ln.BackendInfo, error) {
	return &ln.BackendInfo{
		NodePubKey:    "02mock0000000000000000000000000000000000000000000000000000000000",
		Alias:         "mock-node",
		Network:       "regtest",
		SyncedToChain: true,
		Balance:       1000000,
	}, nil
}

// Start implements the ln.Backend interface.
func (m *MockLNBackend) Start(ctx context.Context) error {
	return nil
}

// Stop implements the ln.Backend interface.
func (m *MockLNBackend) Stop() error {
	return nil
}

// SetPaymentResult configures the result for a specific invoice.
func (m *MockLNBackend) SetPaymentResult(
	invoice string, preimage lntypes.Preimage, err error) {

	m.PaymentResults[invoice] = paymentResult{
		Preimage: preimage,
		Err:      err,
	}
}

// GetPayments returns all payments made to this backend.
func (m *MockLNBackend) GetPayments() []Payment {
	return m.Payments
}

// ResetPayments clears the recorded payments.
func (m *MockLNBackend) ResetPayments() {
	m.Payments = make([]Payment, 0)
}
