---
name: lnd
description: Install and run lnd Lightning Network daemon natively with neutrino backend and SQLite storage. Use when setting up a Lightning node for payments, managing wallets, opening channels, paying invoices, or enabling an agent to send/receive Lightning payments for L402 commerce.
---

# LND Lightning Network Node

Install and operate an lnd Lightning Network node for agent-driven payments.
Defaults to neutrino (light client) backend with SQLite storage for minimal
setup — no full Bitcoin node required.

## Quick Start

```bash
# 1. Install lnd
SKILL_DIR="$(dirname "$(readlink -f "$0")" 2>/dev/null || echo "skills/lnd")"
"${SKILL_DIR}/scripts/install.sh"

# 2. Create encrypted wallet
"${SKILL_DIR}/scripts/create-wallet.sh"

# 3. Start lnd
"${SKILL_DIR}/scripts/start-lnd.sh"

# 4. Check status
"${SKILL_DIR}/scripts/lncli.sh" getinfo
```

## Installation

The install script builds lnd from source with all required build tags:

```bash
skills/lnd/scripts/install.sh
```

This will:
- Verify Go is installed (required)
- Run `go install` with tags: `signrpc walletrpc chainrpc invoicesrpc routerrpc
  peersrpc kvdb_sqlite neutrinorpc`
- Verify `lnd` and `lncli` are on `$PATH`

To install manually:

```bash
go install -tags "signrpc walletrpc chainrpc invoicesrpc routerrpc peersrpc kvdb_sqlite neutrinorpc" github.com/lightningnetwork/lnd/cmd/lnd@latest
go install -tags "signrpc walletrpc chainrpc invoicesrpc routerrpc peersrpc kvdb_sqlite neutrinorpc" github.com/lightningnetwork/lnd/cmd/lncli@latest
```

## Wallet Setup

### Create an Encrypted Wallet

```bash
skills/lnd/scripts/create-wallet.sh
```

This handles the full wallet creation flow:

1. Generates a secure random wallet passphrase
2. Starts lnd temporarily (if not running)
3. Calls `lncli create` with the passphrase
4. Captures and stores the 24-word seed mnemonic
5. Stores credentials securely:
   - `~/.lnget/lnd/wallet-password.txt` (mode 0600) — wallet unlock passphrase
   - `~/.lnget/lnd/seed.txt` (mode 0600) — 24-word recovery mnemonic

**Options:**

```bash
# Custom data directory
create-wallet.sh --lnddir ~/.lnd-agent

# Specific network
create-wallet.sh --network mainnet

# Custom passphrase (instead of auto-generated)
create-wallet.sh --password "your-passphrase-here"
```

> **Security note:** The seed mnemonic on disk is a temporary convenience for
> agent automation. For production use with real funds, migrate to lnd's remote
> signer mode where the seed is held on a separate signing device. See
> [references/security.md](references/security.md) for details.

### Unlock Wallet

After lnd restarts, the wallet must be unlocked before the node is operational:

```bash
skills/lnd/scripts/unlock-wallet.sh
```

This reads the passphrase from `~/.lnget/lnd/wallet-password.txt` and calls the
lnd REST API to unlock. Alternatively, lnd can auto-unlock on start using the
`wallet-unlock-password-file` config option (included in the default template).

### Recover Wallet from Seed

```bash
skills/lnd/scripts/create-wallet.sh --recover --seed-file ~/.lnget/lnd/seed.txt
```

## Starting and Stopping

### Start lnd

```bash
skills/lnd/scripts/start-lnd.sh
```

Starts lnd as a background process using the config at `~/.lnget/lnd/lnd.conf`.
Defaults:
- **Backend:** neutrino (BIP 157/158 light client)
- **Database:** SQLite
- **Network:** mainnet (override with `--network testnet`)
- **Auto-unlock:** enabled via password file

**Options:**

```bash
# Specify network
start-lnd.sh --network testnet

# Custom lnd directory
start-lnd.sh --lnddir ~/.lnd-agent

# Foreground mode (for debugging)
start-lnd.sh --foreground

# With extra lnd flags
start-lnd.sh --extra-args "--debuglevel=trace"
```

### Stop lnd

```bash
skills/lnd/scripts/stop-lnd.sh
```

Gracefully stops lnd via `lncli stop`. Falls back to SIGTERM if lncli fails.

## Node Operations

All commands go through the lncli wrapper which auto-detects paths and network:

### Node Info

```bash
# Get node status
skills/lnd/scripts/lncli.sh getinfo

# Wallet balance (on-chain)
skills/lnd/scripts/lncli.sh walletbalance

# Channel balance (Lightning)
skills/lnd/scripts/lncli.sh channelbalance
```

### Funding the Wallet

```bash
# Generate a new address
skills/lnd/scripts/lncli.sh newaddress p2tr

# Check balance after sending funds
skills/lnd/scripts/lncli.sh walletbalance
```

For testnet, use a faucet. For mainnet, send BTC to the generated address.

### Channel Management

```bash
# Connect to a peer
skills/lnd/scripts/lncli.sh connect <pubkey>@<host>:9735

# Open a channel (satoshis)
skills/lnd/scripts/lncli.sh openchannel --node_key=<pubkey> --local_amt=1000000

# List channels
skills/lnd/scripts/lncli.sh listchannels

# Check channel balance
skills/lnd/scripts/lncli.sh channelbalance

# Close channel cooperatively
skills/lnd/scripts/lncli.sh closechannel --funding_txid=<txid> --output_index=<n>
```

### Payments

```bash
# Create an invoice
skills/lnd/scripts/lncli.sh addinvoice --amt=1000 --memo="test payment"

# Decode a BOLT11 invoice
skills/lnd/scripts/lncli.sh decodepayreq <bolt11_invoice>

# Pay an invoice
skills/lnd/scripts/lncli.sh sendpayment --pay_req=<bolt11_invoice>

# List payments
skills/lnd/scripts/lncli.sh listpayments

# List received invoices
skills/lnd/scripts/lncli.sh listinvoices
```

### Peer Management

```bash
# List connected peers
skills/lnd/scripts/lncli.sh listpeers

# Disconnect from peer
skills/lnd/scripts/lncli.sh disconnect <pubkey>
```

## Configuration

The default config template lives at `skills/lnd/templates/lnd.conf.template`.
On first run, `start-lnd.sh` copies it to `~/.lnget/lnd/lnd.conf`.

Key defaults:

```ini
[Application Options]
alias=lnget-agent
listen=0.0.0.0:9735
rpclisten=localhost:10009
restlisten=localhost:8080
wallet-unlock-password-file=~/.lnget/lnd/wallet-password.txt
wallet-unlock-allow-create=true

[Bitcoin]
bitcoin.active=true
bitcoin.mainnet=true
bitcoin.node=neutrino

[neutrino]
neutrino.addpeer=btcd0.lightning.computer
neutrino.addpeer=mainnet1-btcd.zaphq.io
neutrino.addpeer=mainnet2-btcd.zaphq.io
neutrino.feeurl=https://nodes.lightning.computer/fees/v1/btc-fee-estimates.json

[db]
db.backend=sqlite
```

Override network:

```bash
# For testnet
start-lnd.sh --network testnet
```

## Ports

| Port  | Service   | Description                    |
|-------|-----------|--------------------------------|
| 9735  | Lightning | Peer-to-peer Lightning Network |
| 10009 | gRPC      | lncli and programmatic access  |
| 8080  | REST      | REST API (wallet unlock, etc.) |

## File Locations

| Path | Purpose |
|------|---------|
| `~/.lnget/lnd/lnd.conf` | Configuration file |
| `~/.lnget/lnd/wallet-password.txt` | Wallet unlock passphrase (0600) |
| `~/.lnget/lnd/seed.txt` | 24-word mnemonic backup (0600) |
| `~/.lnd/` | lnd data directory (default) |
| `~/.lnd/data/chain/bitcoin/<network>/` | Chain data and macaroons |
| `~/.lnd/tls.cert` | TLS certificate |
| `~/.lnd/tls.key` | TLS private key |
| `~/.lnd/logs/` | Log files |

## Integration with lnget

Once lnd is running with a funded wallet and open channels, configure lnget to
use it:

```bash
# Initialize lnget config
lnget config init

# lnget auto-detects lnd at localhost:10009 with default paths
lnget ln status

# Fetch an L402-protected resource
lnget --max-cost 1000 https://api.example.com/paid-data
```

Or set config explicitly:

```yaml
# ~/.lnget/config.yaml
ln:
  mode: lnd
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnd/tls.cert
    macaroon: ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
    network: mainnet
```

## Security Considerations

See [references/security.md](references/security.md) for detailed security
guidance.

**Current model (convenience):**
- Wallet passphrase stored on disk at `~/.lnget/lnd/wallet-password.txt`
- Seed mnemonic stored on disk at `~/.lnget/lnd/seed.txt`
- Both files created with mode 0600 (owner read/write only)
- Suitable for testnet, small amounts, and agent automation

**Future improvements:**
- Remote signer mode (seed never on the agent's machine)
- Hardware signing device integration
- Encrypted credential storage with OS keychain
- Multi-party signing for high-value operations

## Troubleshooting

### "wallet not found"
Run `skills/lnd/scripts/create-wallet.sh` to create the wallet first.

### "wallet locked"
Run `skills/lnd/scripts/unlock-wallet.sh` or restart lnd (auto-unlock is
enabled by default in the config template).

### "chain backend is still syncing"
Neutrino needs time to sync headers. Check progress with:
```bash
skills/lnd/scripts/lncli.sh getinfo | jq '{synced_to_chain, block_height}'
```

### "unable to find a path to destination"
No route exists. Check channel balances:
```bash
skills/lnd/scripts/lncli.sh listchannels | jq '.[].channels[] | {remote_pubkey, local_balance, remote_balance}'
```

### "connect: connection refused" on lncli
lnd is not running or not listening. Check:
```bash
skills/lnd/scripts/lncli.sh --help  # Verify lncli works
pgrep lnd                           # Check if lnd process exists
```
