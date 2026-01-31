# lnget Agent Guide

This document is for AI agents working on the lnget codebase. It covers architecture, key abstractions, common modification patterns, and pitfalls to avoid.

## Project Overview

lnget is a curl/wget-like CLI that handles L402 Lightning payments transparently. When a server responds with HTTP 402 Payment Required, lnget:

1. Parses the `WWW-Authenticate: L402 macaroon="...", invoice="..."` header
2. Pays the Lightning invoice via a configured backend (lnd, LNC, or neutrino)
3. Caches the token (macaroon + preimage) per-domain
4. Retries the request with `Authorization: L402 <macaroon>:<preimage>`

## Architecture

```
cmd/lnget/main.go      Entry point - just calls cli.Execute()
        │
        ▼
    cli/root.go        Main download command with wget/curl-like flags
    cli/config.go      lnget config subcommand (init, show, set, path)
    cli/tokens.go      lnget tokens subcommand (list, show, remove, clear)
    cli/ln.go          lnget ln subcommand (status, info, lnc, neutrino)
        │
        ▼
    client/            HTTP client layer
    ├── client.go      Download orchestration, progress tracking
    ├── transport.go   L402Transport - http.RoundTripper that handles 402s
    ├── progress.go    Terminal progress bar rendering
    ├── resume.go      Range header support for resuming downloads
    └── output.go      JSON/human output formatting
        │
        ▼
    l402/              L402 protocol handling
    ├── handler.go     Challenge detection and payment coordination
    ├── header.go      WWW-Authenticate and Authorization header parsing
    ├── store.go       Store interface for token persistence
    └── filestore.go   File-based store at ~/.lnget/tokens/<domain>/
        │
        ▼
    ln/                Lightning backends
    ├── interface.go   Backend interface (Start, Stop, PayInvoice, GetInfo)
    ├── lnd.go         External lnd via lndclient
    ├── lnc.go         Lightning Node Connect
    ├── neutrino.go    Embedded neutrino SPV wallet
    └── session.go     LNC session persistence
        │
        ▼
    config/            Configuration management
    ├── config.go      Config struct, YAML loading, validation
    └── defaults.go    Default values
        │
        ▼
    build/             Build-time info
    ├── version.go     Git commit, version, build date
    └── log.go         btclog-based structured logging
```

## Key Abstractions

### L402Transport (client/transport.go)

The core of lnget's L402 handling. Implements `http.RoundTripper`:

```go
type L402Transport struct {
    base    http.RoundTripper  // Underlying transport (usually http.DefaultTransport)
    handler *l402.Handler      // Handles payment flow
}
```

On every request:
1. Check if we have a cached token for this domain
2. If yes, add the Authorization header
3. Make the request
4. If 402 response with L402 challenge, trigger payment flow
5. Retry with paid token

### Handler (l402/handler.go)

Coordinates the payment flow:

```go
type Handler struct {
    store   Store      // Token persistence
    backend ln.Backend // Lightning payment
    config  *Config    // Max cost, max fee, timeout
}
```

Key method: `HandleChallenge(ctx, url, challenge) (*Token, error)`

1. Parse the challenge header
2. Check invoice amount against max cost
3. Store pending token (before payment, in case of crash)
4. Pay invoice via backend
5. Store paid token with preimage
6. Return token for retry

### Store Interface (l402/store.go)

```go
type Store interface {
    GetToken(domain string) (*Token, error)
    StorePending(domain string, token *Token) error
    StoreToken(domain string, token *Token) error
    RemoveToken(domain string) error
    ListTokens() (map[string]*Token, error)
}
```

The `FileStore` implementation stores tokens at `~/.lnget/tokens/<domain>/token.json`.

### Backend Interface (ln/interface.go)

```go
type Backend interface {
    Start(ctx context.Context) error
    Stop() error
    PayInvoice(ctx context.Context, invoice string, maxFeeSat int64,
               timeout time.Duration) (*PaymentResult, error)
    GetInfo(ctx context.Context) (*NodeInfo, error)
}
```

Three implementations:
- `LNDBackend` - connects to external lnd via gRPC
- `LNCBackend` - connects via Lightning Node Connect
- `NeutrinoBackend` - embedded SPV wallet (experimental)

## Common Modification Patterns

### Adding a new CLI flag

1. Add field to the appropriate struct in `cli/root.go`:
```go
type downloadFlags struct {
    // ... existing fields
    newFlag string
}
```

2. Register the flag in `init()`:
```go
cmd.Flags().StringVar(&flags.newFlag, "new-flag", "", "Description")
```

3. Use the value in `runDownload()`:
```go
if flags.newFlag != "" {
    // handle it
}
```

### Adding a new subcommand

1. Create a new file in `cli/`, e.g., `cli/newcmd.go`
2. Define the command:
```go
func newNewCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "newcmd",
        Short: "Does something new",
        RunE:  runNewCmd,
    }
}
```

3. Register in `cli/root.go`'s `init()`:
```go
rootCmd.AddCommand(newNewCmd())
```

### Adding a new Lightning backend

1. Create a new file in `ln/`, e.g., `ln/newbackend.go`
2. Implement the `Backend` interface
3. Add the backend type to config validation in `config/config.go`
4. Add construction logic in `cli/root.go`'s `createBackend()` function

### Modifying token storage format

The token format is defined in `l402/store.go`:

```go
type Token struct {
    Macaroon    []byte
    PaymentHash []byte
    Preimage    []byte
    AmountMsat  int64
    CreatedAt   time.Time
    // ...
}
```

If you change this struct, consider backwards compatibility with existing stored tokens.

## Testing Strategy

### Unit tests

Each package has `*_test.go` files. Run with:

```bash
make unit pkg=l402           # Test l402 package
make unit pkg=client case=TestDownload  # Specific test
make unit log="stdlog trace" pkg=l402   # With debug logs
```

### Test patterns

- **Table-driven tests**: Most tests use `[]struct{ name string; ... }` pattern
- **Mock backends**: `l402/handler_test.go` has `mockBackend` for testing payment flow
- **HTTP test servers**: `httptest.NewServer` for testing client behavior

### Coverage

Aim for ~90% coverage. Check with:

```bash
go test -coverprofile=coverage.txt ./...
go tool cover -html=coverage.txt
```

## Code Style

### Structured logging

Use structured log methods (ending in `S`) with static messages:

```go
// WRONG
log.Infof("Processing request for %s with amount %d", url, amount)

// RIGHT
log.InfoS(ctx, "Processing request",
    slog.String("url", url),
    slog.Int64("amount_msat", amount))
```

### Error handling

Never use inline error handling:

```go
// WRONG
if err := doSomething(); err != nil {
    return err
}

// RIGHT
err := doSomething()
if err != nil {
    return err
}
```

### Function comments

Every function needs a comment starting with the function name:

```go
// PayInvoice pays the given invoice using the configured backend.
// It returns the payment result including the preimage.
func (h *Handler) PayInvoice(ctx context.Context, invoice string) (*PaymentResult, error) {
```

### Line length

Keep lines under 80 characters where practical. Use 8-space tabs.

## Common Pitfalls

### 1. Paying without checking amount

Always verify invoice amount against `--max-cost` before paying:

```go
if invoiceAmount > h.config.MaxCostSats {
    return nil, ErrExceedsMaxCost
}
```

### 2. Losing pending tokens

Store the token BEFORE initiating payment:

```go
// Store pending first
err := store.StorePending(domain, token)
if err != nil {
    return err
}

// Then pay
result, err := backend.PayInvoice(...)
if err != nil {
    // Token is still stored as pending, can be recovered
    return err
}

// Update with preimage
token.Preimage = result.Preimage
err = store.StoreToken(domain, token)
```

### 3. Using error log level incorrectly

Only use `error` level for internal bugs, not external failures:

```go
// WRONG - payment failure is external
log.ErrorS(ctx, "Payment failed", slog.String("err", err.Error()))

// RIGHT - use warn for external failures
log.WarnS(ctx, "Payment failed", slog.String("err", err.Error()))
```

### 4. Forgetting to run linters

Before every commit:

```bash
make lint    # Must pass with 0 issues
make unit    # Tests must pass
```

### 5. Breaking backwards compatibility

The config file format and token storage format should remain backwards compatible. If you must change them, add migration logic.

## Dependency Notes

### Core dependencies

- **aperture/l402**: Reference L402 implementation from Lightning Labs
- **lndclient**: gRPC client for lnd
- **btcd/btcutil**: Bitcoin utilities
- **cobra**: CLI framework
- **viper**: Configuration management

### Updating dependencies

```bash
go get -u github.com/some/dependency@latest
make tidy-module-check  # Verify go.mod is clean
```

## Directory Layout Reference

```
~/.lnget/
├── config.yaml           # Main config file
└── tokens/
    └── api.example.com/
        └── token.json    # Cached L402 token
```

## Exit Codes

When implementing error handling, use the correct exit codes:

| Code | Constant | Usage |
|------|----------|-------|
| 0 | - | Success |
| 1 | ExitError | General error |
| 2 | ExitPaymentExceedsMax | Invoice amount > max-cost |
| 3 | ExitPaymentFailed | Lightning payment failed |
| 4 | ExitNetworkError | HTTP/connection error |

## Quick Reference

### Build and test
```bash
make build       # Build binary
make lint        # Run linters
make unit        # Run tests
make fmt         # Format code
```

### Debug with logs
```bash
make unit log="stdlog trace" pkg=l402 case=TestHandler
```

### Check a specific file's tests
```bash
go test -v ./l402 -run TestParseChallenge
```

### View go doc for a package
```bash
go doc github.com/lightninglabs/lnget/l402
go doc github.com/lightninglabs/lnget/l402.Handler
```
