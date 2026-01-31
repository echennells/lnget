# Troubleshooting Guide

Common issues and their solutions when using lnget.

## Quick Diagnostic Commands

```bash
# Check Lightning backend connection
lnget ln status

# Verify config
lnget config show

# List cached tokens
lnget tokens list

# Test with verbose output
lnget -v https://api.example.com/data.json
```

## Connection Issues

### "connection refused" or "cannot connect to lnd"

**Symptoms:**
```
Error: failed to connect to lnd: connection refused
```

**Causes and fixes:**

1. **lnd not running**
   ```bash
   # Check if lnd is running
   pgrep lnd
   # Start lnd
   lnd
   ```

2. **Wrong host/port**
   ```bash
   # Check lnd's actual listen address
   cat ~/.lnd/lnd.conf | grep rpclisten
   # Update lnget config to match
   lnget config show | grep host
   ```

3. **Firewall blocking**
   ```bash
   # Test connectivity
   nc -zv localhost 10009
   ```

### "certificate signed by unknown authority"

**Symptoms:**
```
Error: transport: authentication handshake failed: x509: certificate signed by unknown authority
```

**Causes and fixes:**

1. **Wrong TLS cert path**
   ```bash
   # Verify cert exists
   ls -la ~/.lnd/tls.cert
   # Check config path
   lnget config show | grep tls
   ```

2. **lnd regenerated its certificate**
   ```bash
   # Copy fresh cert
   cp ~/.lnd/tls.cert ~/.lnget/tls.cert
   ```

3. **Cert doesn't match host**
   ```bash
   # Check cert's CN/SAN
   openssl x509 -in ~/.lnd/tls.cert -noout -text | grep -A1 "Subject:"
   ```

### "macaroon authentication failed"

**Symptoms:**
```
Error: permission denied: macaroon authentication failed
```

**Causes and fixes:**

1. **Wrong macaroon path**
   ```bash
   # Verify macaroon exists
   ls -la ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon
   ```

2. **Corrupted macaroon**
   ```bash
   # Check it's valid binary
   xxd ~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon | head -1
   # Should show binary data, not text
   ```

3. **Macaroon for wrong network**
   ```bash
   # Ensure network matches
   # mainnet: data/chain/bitcoin/mainnet/
   # testnet: data/chain/bitcoin/testnet/
   # regtest: data/chain/bitcoin/regtest/
   ```

## Payment Issues

### "payment failed: no route"

**Symptoms:**
```
Error: payment failed: no route found
```

**Causes and fixes:**

1. **No channels**
   ```bash
   # Check channels
   lncli listchannels | jq '.channels | length'
   # Need at least one open channel
   ```

2. **Insufficient outbound capacity**
   ```bash
   # Check local balance
   lncli listchannels | jq '.channels[] | {remote_pubkey, local_balance}'
   # local_balance must exceed invoice amount
   ```

3. **Network not synced**
   ```bash
   # Check sync status
   lncli getinfo | jq '{synced_to_chain, synced_to_graph}'
   # Both should be true
   ```

4. **No path to destination**
   - Target node may be poorly connected
   - Try with higher `--max-fee` to allow more routing options

### "payment failed: insufficient balance"

**Symptoms:**
```
Error: payment failed: insufficient local balance
```

**Fix:**
- Need more outbound liquidity
- Open a new channel or rebalance existing channels

### "payment would exceed max cost"

**Symptoms:**
```
Error: invoice amount 5000 sat exceeds max cost 1000 sat
Exit code: 2
```

**Fix:**
```bash
# Increase the limit
lnget --max-cost 5000 https://api.example.com/data.json

# Or preview first
lnget --no-pay https://api.example.com/data.json
```

### "payment timeout"

**Symptoms:**
```
Error: payment timed out after 60s
```

**Causes and fixes:**

1. **Network congestion**
   ```bash
   # Increase timeout
   lnget --payment-timeout 5m https://api.example.com/data.json
   ```

2. **Invoice already expired**
   - Request new invoice (clear token, retry)
   ```bash
   lnget tokens remove api.example.com
   lnget https://api.example.com/data.json
   ```

## Token Issues

### "token not accepted by server"

**Symptoms:**
```
Error: 401 Unauthorized after presenting token
```

**Causes and fixes:**

1. **Token expired**
   ```bash
   # Check token age
   lnget tokens show api.example.com
   # Clear and get new token
   lnget tokens remove api.example.com
   ```

2. **Token for wrong domain**
   ```bash
   # Tokens are domain-scoped
   # api.example.com != www.api.example.com
   lnget tokens list
   ```

3. **Server-side invalidation**
   - Server may have revoked the macaroon
   - Clear token and pay again

### "token file corrupted"

**Symptoms:**
```
Error: failed to load token: invalid character
```

**Fix:**
```bash
# Remove corrupted token
lnget tokens remove api.example.com
# Or clear all
lnget tokens clear --force
```

### Tokens not being reused

**Symptoms:**
- Paying for every request to same domain

**Check:**
```bash
# Verify token exists
lnget tokens show api.example.com

# Check domain matches exactly
# URL: https://api.example.com/data
# Token stored for: api.example.com
```

## HTTP Issues

### "connection reset" or "EOF"

**Symptoms:**
```
Error: connection reset by peer
```

**Causes:**
- Server closed connection unexpectedly
- Network instability
- Server-side timeout

**Fixes:**
```bash
# Retry
lnget https://api.example.com/data.json

# With verbose for debugging
lnget -v https://api.example.com/data.json
```

### "TLS handshake timeout"

**Symptoms:**
```
Error: net/http: TLS handshake timeout
```

**Causes:**
- Slow network
- Server overloaded

**Fixes:**
```bash
# Increase timeout in config
# http:
#   timeout: 60s
```

### "certificate verify failed" (HTTPS)

**Symptoms:**
```
Error: x509: certificate verify failed
```

**Causes:**
- Self-signed cert on server
- Expired cert
- Wrong hostname

**Fix (for testing only):**
```bash
# Skip TLS verification (INSECURE)
lnget -k https://api.example.com/data.json
```

### Redirects not followed

**Symptoms:**
- Getting 301/302 response instead of data

**Check:**
```bash
# Redirects are followed by default
# If disabled in config, re-enable:
lnget -L https://api.example.com/data.json
```

## Configuration Issues

### Config file not found

**Symptoms:**
```
Warning: config file not found, using defaults
```

**Fix:**
```bash
# Create default config
lnget config init

# Check path
lnget config path
```

### Environment variables not working

**Check:**
```bash
# Variables must be prefixed with LNGET_
# Nested keys use underscore
export LNGET_LN_LND_HOST=localhost:10009
export LNGET_L402_MAX_COST_SATS=5000

# Verify
env | grep LNGET
```

### Config changes not taking effect

**Check:**
- Environment variables override config file
- Command-line flags override both

```bash
# See effective config
lnget config show

# Check for env overrides
env | grep LNGET
```

## Resume Issues

### Resume not working

**Symptoms:**
- Download starts from beginning instead of resuming

**Causes:**
1. **Server doesn't support Range requests**
   ```bash
   # Check server support
   curl -I https://api.example.com/file.zip | grep -i accept-ranges
   # Should show: Accept-Ranges: bytes
   ```

2. **File was modified**
   - Server file changed since partial download
   - Start fresh

3. **Missing -c flag**
   ```bash
   # Must use -c to resume
   lnget -c -o file.zip https://api.example.com/file.zip
   ```

## Debugging Tips

### Enable verbose output

```bash
lnget -v https://api.example.com/data.json
```

Shows:
- Request/response headers
- L402 challenge details
- Payment progress

### Check exact error

```bash
# Capture stderr
lnget https://api.example.com/data.json 2>error.log
cat error.log
```

### Test components separately

```bash
# 1. Test Lightning connection
lnget ln status
lnget ln info

# 2. Test HTTP without payment
curl -v https://api.example.com/data.json

# 3. Test with payment preview
lnget --no-pay https://api.example.com/data.json
```

### Check exit codes

```bash
lnget https://api.example.com/data.json
echo "Exit code: $?"

# 0 = success
# 1 = general error
# 2 = payment exceeds max cost
# 3 = payment failed
# 4 = network error
```

## Getting Help

If issues persist:

1. Run with verbose: `lnget -v ...`
2. Check lnd logs: `tail -f ~/.lnd/logs/bitcoin/mainnet/lnd.log`
3. Verify network: `lncli getinfo`
4. Open issue with output at: https://github.com/lightninglabs/lnget/issues
