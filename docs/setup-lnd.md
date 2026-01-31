# Setting Up lnd for lnget

This guide covers configuring lnd as the Lightning backend for lnget.

## Prerequisites

- A running lnd node (v0.20.0 or later required)
- Access to lnd's admin macaroon and TLS certificate
- lnd must have sufficient outbound liquidity for payments

## Quick Setup

### 1. Locate your lnd credentials

lnd stores credentials in its data directory. Common locations:

```bash
# Linux
~/.lnd/

# macOS
~/Library/Application Support/Lnd/

# Custom (check your lnd.conf)
cat ~/.lnd/lnd.conf | grep -E "^(lnddir|datadir)"
```

You need two files:
- **TLS certificate**: `tls.cert` (in lnd directory root)
- **Admin macaroon**: `data/chain/bitcoin/<network>/admin.macaroon`

Where `<network>` is `mainnet`, `testnet`, `signet`, or `regtest`.

### 2. Initialize lnget config

```bash
lnget config init
```

This creates `~/.lnget/config.yaml`.

### 3. Configure lnd connection

Edit `~/.lnget/config.yaml`:

```yaml
ln:
  mode: lnd
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnd/tls.cert
    macaroon: ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
    network: mainnet
```

Or use environment variables:

```bash
export LNGET_LN_MODE=lnd
export LNGET_LN_LND_HOST=localhost:10009
export LNGET_LN_LND_TLS_CERT=~/.lnd/tls.cert
export LNGET_LN_LND_MACAROON=~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
```

### 4. Verify connection

```bash
lnget ln status
```

Expected output:
```
Backend: lnd
Status: connected
Pubkey: 03abc123...
Alias: MyNode
Network: mainnet
```

For detailed node info:
```bash
lnget ln info
```

## Remote lnd Access

### Using SSH tunnel

If lnd runs on a remote server:

```bash
# Create SSH tunnel
ssh -L 10009:localhost:10009 user@remote-server -N &

# Copy credentials locally
scp user@remote-server:~/.lnd/tls.cert ~/.lnget/
scp user@remote-server:~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon ~/.lnget/
```

Configure lnget to use local paths:
```yaml
ln:
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnget/tls.cert
    macaroon: ~/.lnget/admin.macaroon
```

### Using Tor

If lnd exposes a Tor hidden service:

```yaml
ln:
  lnd:
    host: your-onion-address.onion:10009
    tls_cert: /path/to/tls.cert
    macaroon: /path/to/admin.macaroon
```

Note: lnget uses the system's Tor proxy if available.

## Minimal Macaroon Permissions

The admin macaroon works but has more permissions than needed. For production, bake a custom macaroon with only required permissions:

```bash
lncli bakemacaroon \
    invoices:read \
    invoices:write \
    offchain:read \
    offchain:write \
    info:read \
    --save_to ~/.lnget/lnget.macaroon
```

Then use this macaroon in your config:
```yaml
ln:
  lnd:
    macaroon: ~/.lnget/lnget.macaroon
```

## Troubleshooting

### "connection refused"

lnd isn't running or isn't listening on the configured port:
```bash
# Check if lnd is running
pgrep lnd

# Check what port lnd is using
cat ~/.lnd/lnd.conf | grep rpclisten
```

### "certificate signed by unknown authority"

TLS certificate mismatch. Make sure you're using the correct `tls.cert`:
```bash
# Verify cert matches lnd
openssl x509 -in ~/.lnd/tls.cert -noout -text | grep -A1 "Subject:"
```

If lnd regenerated its cert, copy the new one.

### "permission denied" or "macaroon invalid"

Macaroon issues:
```bash
# Check macaroon file exists and is readable
ls -la ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon

# Verify it's a valid macaroon (should output binary)
xxd ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon | head -1
```

### "insufficient balance"

Your node doesn't have enough outbound capacity:
```bash
# Check channel balances
lncli listchannels | jq '.channels[] | {remote_pubkey, local_balance, remote_balance}'
```

You need outbound liquidity (local_balance) to make payments.

## Testing with Regtest

For development, run lnd on regtest:

```bash
# Start bitcoind in regtest
bitcoind -regtest -daemon

# Start lnd
lnd --bitcoin.active --bitcoin.regtest --bitcoin.node=bitcoind

# Create wallet
lncli create

# Generate blocks and fund wallet
bitcoin-cli -regtest generatetoaddress 101 $(lncli newaddress p2wkh | jq -r .address)
```

Configure lnget for regtest:
```yaml
ln:
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnd/tls.cert
    macaroon: ~/.lnd/data/chain/bitcoin/regtest/admin.macaroon
    network: regtest
```

## Using Polar

[Polar](https://lightningpolar.com/) provides a GUI for running local Lightning networks:

1. Download and install Polar
2. Create a network with lnd nodes
3. Start the network
4. Click on an lnd node → "Connect" tab
5. Copy the connection details to your lnget config

Polar automatically manages credentials and makes them easy to export.

## Next Steps

- [Setting up Aperture](setup-aperture.md) - Run an L402 server for testing
- [Workflow Examples](workflows.md) - Common usage patterns
