# Embedded Neutrino Wallet Setup

lnget includes an experimental embedded Lightning wallet using Neutrino, a lightweight SPV (Simplified Payment Verification) client. This enables lnget to make payments without requiring a separate lnd node.

> **Note**: The embedded neutrino wallet is experimental. For production use, we recommend connecting to an external lnd node.

## What is Neutrino?

Neutrino is a lightweight Bitcoin client that:
- Downloads only block headers and filters (not full blocks)
- Requires ~500MB instead of ~500GB for full node
- Syncs in minutes instead of days
- Can verify transactions relevant to your wallet

Combined with an embedded lnd wallet, this gives lnget self-contained payment capability.

## When to Use Neutrino

**Good for:**
- Quick testing without existing Lightning infrastructure
- Single-machine setups
- Users without access to an lnd node
- Development and experimentation

**Not recommended for:**
- Production/commercial use
- High-volume payments
- Privacy-critical applications (SPV has privacy tradeoffs)

## Setup

### 1. Initialize the wallet

```bash
lnget ln neutrino init
```

This will:
1. Create wallet directory at `~/.lnget/neutrino/`
2. Generate a new seed phrase
3. Create an encrypted wallet

**Important**: Write down the seed phrase! It's your only backup.

```
Initializing neutrino wallet...

Seed phrase (WRITE THIS DOWN):
  1. abandon  2. ability  3. able    4. about
  5. above    6. absent   7. absorb  8. abstract
  ...

Wallet initialized at ~/.lnget/neutrino/
```

### 2. Configure lnget to use neutrino

```yaml
ln:
  mode: neutrino
```

Or via environment:
```bash
export LNGET_LN_MODE=neutrino
```

### 3. Start sync and check status

```bash
lnget ln neutrino status
```

```
Neutrino Status:
  Synced: false
  Height: 150000 / 820000
  Progress: 18.3%
  Peers: 8
```

Wait for sync to complete (can take 5-30 minutes depending on network).

### 4. Fund the wallet

Get a receiving address:
```bash
lnget ln neutrino fund
```

```
Send Bitcoin to: bc1q...
Minimum for channel: 0.001 BTC (100,000 sats)
```

Send Bitcoin to this address. After confirmation, lnget can open channels.

### 5. Open a channel

Once funded, open a channel to enable payments:
```bash
lnget ln neutrino open-channel --peer <pubkey@host:port> --amount 500000
```

Or let lnget auto-select a well-connected node:
```bash
lnget ln neutrino auto-channel --amount 500000
```

## Wallet Management

### Check balance

```bash
lnget ln neutrino balance
```

```
On-chain balance: 100000 sats
Channel balance:  450000 sats (local)
                  50000 sats (remote)
Total spendable:  450000 sats
```

### List channels

```bash
lnget ln neutrino channels
```

### Close channel

```bash
lnget ln neutrino close-channel --channel-id <id>
```

### Backup seed

If you lost your seed phrase:
```bash
lnget ln neutrino show-seed
```
(Requires wallet password)

## Configuration

### Full neutrino config

```yaml
ln:
  mode: neutrino
  neutrino:
    # Wallet data directory
    data_dir: ~/.lnget/neutrino

    # Bitcoin network
    network: mainnet  # or testnet, signet, regtest

    # Neutrino peers (optional, uses DNS seeds by default)
    # peers:
    #   - btcd.example.com:8333

    # Wallet password (or prompt if not set)
    # password: "your-wallet-password"

    # Auto-pilot for channel management
    autopilot:
      enabled: true
      max_channels: 3
      allocation: 0.6  # 60% of funds in channels
```

### Environment variables

```bash
export LNGET_LN_MODE=neutrino
export LNGET_LN_NEUTRINO_DATA_DIR=~/.lnget/neutrino
export LNGET_LN_NEUTRINO_NETWORK=mainnet
```

## Sync Process

### Initial sync

First run requires downloading block filters:

```
Mainnet:  ~500MB, 10-30 minutes
Testnet:  ~100MB, 5-10 minutes
Signet:   ~50MB,  2-5 minutes
Regtest:  Instant (local only)
```

### Subsequent starts

After initial sync, startup is fast:
```
Checking for new blocks...
Synced to height 820001
Ready.
```

### Speeding up sync

Use specific peers for faster initial sync:
```yaml
ln:
  neutrino:
    peers:
      - btcd.lnd.engineering:8333
      - mainnet1-btcd.zaphq.io:8333
```

## Making Payments

Once synced with an open channel:

```bash
# Normal lnget usage
lnget https://api.example.com/paid-data.json
```

Payment flows through your embedded wallet's channels.

## Limitations

### Compared to external lnd

| Feature | External lnd | Embedded Neutrino |
|---------|-------------|-------------------|
| Sync time | Depends on backend | 10-30 min initial |
| Disk usage | Backend dependent | ~500MB |
| Channel management | Full control | Basic |
| Routing | Full graph | Limited |
| Reliability | Production-ready | Experimental |
| Privacy | Depends on setup | SPV limitations |

### SPV Privacy Tradeoffs

Neutrino queries full nodes for data. This reveals:
- Which blocks you're interested in
- Addresses you're watching

For privacy-critical applications, use a full node or external lnd.

### Channel Limitations

The embedded wallet has simplified channel management:
- No channel routing (only direct payments to channel peer's network)
- Limited channel capacity management
- Basic fee estimation

## Troubleshooting

### "sync taking too long"

**Causes:**
- Slow network
- Few peers

**Fixes:**
```bash
# Check peer count
lnget ln neutrino status

# If low peer count, add manual peers in config
```

### "cannot open channel: insufficient funds"

**Cause**: On-chain balance too low

**Fix**: Send more Bitcoin to your wallet address:
```bash
lnget ln neutrino fund
```

### "payment failed: no route"

**Causes:**
- Channel not open/confirmed
- Insufficient outbound capacity
- Target not reachable from your peer

**Fixes:**
1. Wait for channel confirmation (6 blocks)
2. Open channel with better-connected node
3. Try with higher `--max-fee`

### "wallet locked"

```bash
# Unlock wallet
lnget ln neutrino unlock
```

Or set password in config/environment for auto-unlock.

### "database corrupted"

If wallet database is corrupted:
```bash
# Backup seed first!
lnget ln neutrino show-seed

# Reset wallet
rm -rf ~/.lnget/neutrino/
lnget ln neutrino init --restore
# Enter your seed phrase
```

## Security

### Wallet encryption

The wallet is encrypted with your password. Choose a strong password.

### Seed phrase

Your seed phrase is the master backup:
- Write it down physically
- Store securely
- Never share it
- With the seed, anyone can recover your funds

### File permissions

lnget sets restrictive permissions:
```bash
ls -la ~/.lnget/neutrino/
# drwx------ wallet.db, etc.
```

## Development/Testing

### Regtest mode

For development, use regtest:

```yaml
ln:
  neutrino:
    network: regtest
```

```bash
# Start local bitcoind in regtest
bitcoind -regtest -daemon

# Initialize wallet
lnget ln neutrino init

# Mine blocks to fund wallet
bitcoin-cli -regtest generatetoaddress 101 $(lnget ln neutrino fund --format address)
```

### Testnet/Signet

For testing with "fake" Bitcoin:

```yaml
ln:
  neutrino:
    network: testnet  # or signet
```

Get testnet coins from a faucet.

## Next Steps

- [Setting up lnd](setup-lnd.md) - Production setup with external lnd
- [LNC setup](lnc-setup.md) - Remote node access
- [Workflows](workflows.md) - Usage examples
