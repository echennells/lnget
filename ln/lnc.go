package ln

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lightning-node-connect/mailbox"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"google.golang.org/grpc"
)

// LNCBackend connects to a Lightning node via Lightning Node Connect.
// This allows connecting to a node through a mailbox relay without direct
// network access.
type LNCBackend struct {
	// grpcConn is the gRPC connection to the Lightning node.
	grpcConn *grpc.ClientConn

	// getConn is a function that returns the gRPC connection.
	getConn func() (*grpc.ClientConn, error)

	// getStatus is a function that returns the connection status.
	getStatus func() mailbox.ClientStatus

	// lnClient is the Lightning client interface.
	lnClient lnrpc.LightningClient

	// routerClient is the router client for payment operations.
	routerClient routerrpc.RouterClient

	// session is the current LNC session.
	session *Session

	// sessionStore manages session persistence.
	sessionStore *SessionStore

	// pairingPhrase is used for initial connection.
	pairingPhrase string

	// mailboxAddr is the mailbox server address.
	mailboxAddr string

	// ephemeral indicates if this is an ephemeral (non-persisted) session.
	ephemeral bool

	// localKey is the local static key for the noise connection.
	localKey keychain.SingleKeyECDH

	// remoteKey is the remote node's public key (learned during handshake).
	remoteKey *btcec.PublicKey

	mu       sync.RWMutex
	started  bool
	stopChan chan struct{}
}

// LNCConfig contains configuration for the LNC backend.
type LNCConfig struct {
	// PairingPhrase is the LNC pairing phrase from the node.
	PairingPhrase string

	// MailboxAddr is the address of the mailbox server.
	// Default: mailbox.terminal.lightning.today:443
	MailboxAddr string

	// SessionStore is used to persist sessions for reconnection.
	SessionStore *SessionStore

	// SessionID is used to resume an existing session.
	SessionID string

	// Ephemeral indicates the session should not be persisted.
	Ephemeral bool

	// LocalKey is the local static key for noise connection.
	// If nil, a new key will be generated.
	LocalKey keychain.SingleKeyECDH
}

// DefaultMailboxAddr is the default Lightning Terminal mailbox server.
const DefaultMailboxAddr = "mailbox.terminal.lightning.today:443"

// NewLNCBackend creates a new LNC backend with the given configuration.
func NewLNCBackend(cfg *LNCConfig) (*LNCBackend, error) {
	if cfg.PairingPhrase == "" && cfg.SessionID == "" {
		return nil, errors.New("pairing phrase or session ID required")
	}

	mailboxAddr := cfg.MailboxAddr
	if mailboxAddr == "" {
		mailboxAddr = DefaultMailboxAddr
	}

	// Ensure the mailbox address is properly formatted.
	if !strings.Contains(mailboxAddr, "://") {
		mailboxAddr = "wss://" + mailboxAddr
	}

	backend := &LNCBackend{
		pairingPhrase: cfg.PairingPhrase,
		mailboxAddr:   mailboxAddr,
		sessionStore:  cfg.SessionStore,
		ephemeral:     cfg.Ephemeral,
		localKey:      cfg.LocalKey,
		stopChan:      make(chan struct{}),
	}

	// If resuming an existing session, load it.
	if cfg.SessionID != "" && cfg.SessionStore != nil {
		session, err := cfg.SessionStore.LoadSession(cfg.SessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to load session: %w", err)
		}

		backend.session = session
		backend.pairingPhrase = session.PairingPhrase
		backend.mailboxAddr = session.MailboxAddr
	}

	return backend, nil
}

// Start initializes the LNC connection to the remote node.
func (l *LNCBackend) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.started {
		return errors.New("LNC backend already started")
	}

	log.Infof("Starting LNC connection to mailbox %s", l.mailboxAddr)

	// Create the websocket connection using the pairing phrase.
	getStatus, getConn, err := mailbox.NewClientWebsocketConn(
		l.mailboxAddr,
		l.pairingPhrase,
		l.localKey,
		l.remoteKey,
		func(key *btcec.PublicKey) error {
			log.Infof("Received remote key: %x",
				key.SerializeCompressed())

			l.remoteKey = key

			return nil
		},
		func(data []byte) error {
			log.Debugf("Received auth data (%d bytes)", len(data))

			return nil
		},
	)
	if err != nil {
		log.Warnf("LNC websocket creation failed: %v", err)

		return fmt.Errorf("failed to create LNC connection: %w", err)
	}

	l.getStatus = getStatus
	l.getConn = getConn

	log.Infof("LNC websocket created, establishing gRPC connection...")

	// Get the gRPC connection with a timeout. The getConn call
	// blocks during the Noise handshake, so without a timeout it
	// can hang indefinitely if the pairing phrase is wrong or the
	// relay is unreachable.
	type connResult struct {
		conn *grpc.ClientConn
		err  error
	}

	connTimeout := 30 * time.Second

	connCh := make(chan connResult, 1)

	go func() {
		c, e := getConn()
		connCh <- connResult{conn: c, err: e}
	}()

	var conn *grpc.ClientConn

	select {
	case res := <-connCh:
		if res.err != nil {
			log.Warnf("gRPC connection failed: %v", res.err)

			return fmt.Errorf("failed to get gRPC "+
				"connection: %w", res.err)
		}

		conn = res.conn

	case <-time.After(connTimeout):
		log.Warnf("gRPC connection timed out after %v", connTimeout)

		return fmt.Errorf("LNC connection timed out after %v "+
			"(check pairing phrase and network)", connTimeout)

	case <-ctx.Done():
		return ctx.Err()
	}

	log.Infof("gRPC connection established")

	l.grpcConn = conn

	// Create the Lightning and Router clients.
	l.lnClient = lnrpc.NewLightningClient(conn)
	l.routerClient = routerrpc.NewRouterClient(conn)

	// Save the session if not ephemeral.
	if !l.ephemeral && l.sessionStore != nil && l.session == nil {
		l.session = &Session{
			ID:            GenerateSessionID(),
			Label:         "lnget-session",
			PairingPhrase: l.pairingPhrase,
			MailboxAddr:   l.mailboxAddr,
			Created:       time.Now(),
			LastUsed:      time.Now(),
		}

		saveErr := l.sessionStore.SaveSession(l.session)
		if saveErr != nil {
			log.Warnf("Failed to save session: %v", saveErr)
		} else {
			log.Infof("Session saved: %s", l.session.ID)
		}
	}

	l.started = true

	log.Infof("LNC backend started successfully")

	return nil
}

// Stop gracefully shuts down the LNC connection.
func (l *LNCBackend) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.started {
		return nil
	}

	close(l.stopChan)

	if l.grpcConn != nil {
		_ = l.grpcConn.Close()
	}

	l.started = false

	return nil
}

// PayInvoice pays the given BOLT11 invoice.
//
//nolint:whitespace
func (l *LNCBackend) PayInvoice(ctx context.Context, invoice string,
	maxFeeSat int64, timeout time.Duration) (*PaymentResult, error) {
	l.mu.RLock()

	if !l.started {
		l.mu.RUnlock()

		return nil, errors.New("LNC backend not started")
	}

	routerClient := l.routerClient
	l.mu.RUnlock()

	// Create a context with the payment timeout.
	payCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Prepare the payment request.
	req := &routerrpc.SendPaymentRequest{
		PaymentRequest: invoice,
		FeeLimitSat:    maxFeeSat,
		TimeoutSeconds: int32(timeout.Seconds()),
	}

	log.Infof("Sending payment via LNC (fee_limit=%d sat, timeout=%v)",
		maxFeeSat, timeout)

	// Send the payment using the router client for better control.
	stream, err := routerClient.SendPaymentV2(payCtx, req)
	if err != nil {
		log.Warnf("Payment initiation failed: %v", err)

		return nil, fmt.Errorf("failed to send payment: %w", err)
	}

	// Wait for the payment result.
	for {
		update, err := stream.Recv()
		if err != nil {
			log.Warnf("Payment stream error: %v", err)

			return nil, fmt.Errorf("payment stream error: %w", err)
		}

		log.Debugf("Payment status update: %s", update.Status)

		switch update.Status {
		case lnrpc.Payment_SUCCEEDED:
			preimage, err := lntypes.MakePreimageFromStr(
				update.PaymentPreimage,
			)
			if err != nil {
				return nil, fmt.Errorf("invalid preimage: %w",
					err)
			}

			return &PaymentResult{
				Preimage: preimage,
				AmountPaid: lnwire.MilliSatoshi(
					update.ValueMsat,
				),
				RoutingFeePaid: lnwire.MilliSatoshi(
					update.FeeMsat,
				),
			}, nil

		case lnrpc.Payment_FAILED:
			return nil, fmt.Errorf("payment failed: %s",
				update.FailureReason)

		case lnrpc.Payment_IN_FLIGHT:
			// Payment still in progress, continue waiting.
			continue

		default:
			// Unknown status, continue.
			continue
		}
	}
}

// GetInfo returns information about the connected Lightning node.
func (l *LNCBackend) GetInfo(ctx context.Context) (*BackendInfo, error) {
	l.mu.RLock()

	if !l.started {
		l.mu.RUnlock()

		return nil, errors.New("LNC backend not started")
	}

	lnClient := l.lnClient
	l.mu.RUnlock()

	info, err := lnClient.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get info: %w", err)
	}

	// Get wallet balance.
	balance, err := lnClient.WalletBalance(ctx, &lnrpc.WalletBalanceRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return &BackendInfo{
		NodePubKey:    info.IdentityPubkey,
		Alias:         info.Alias,
		Network:       info.Chains[0].Network,
		SyncedToChain: info.SyncedToChain,
		Balance:       balance.TotalBalance,
	}, nil
}

// Session returns the current session, if any.
func (l *LNCBackend) Session() *Session {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.session
}

// Status returns the current connection status.
func (l *LNCBackend) Status() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.getStatus == nil {
		return "not initialized"
	}

	return string(l.getStatus())
}
