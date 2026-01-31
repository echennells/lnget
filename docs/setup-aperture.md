# Setting Up Aperture for L402 Testing

Aperture is Lightning Labs' L402 reverse proxy. It sits in front of your API and handles payment verification. This guide covers running Aperture to test lnget against real L402 flows.

## What is Aperture?

Aperture acts as a gateway:

```
Client (lnget) → Aperture → Your Backend API
                   ↓
              lnd (invoices)
```

1. Client requests a protected resource
2. Aperture returns 402 with L402 challenge (macaroon + invoice)
3. Client pays the invoice
4. Client retries with Authorization header
5. Aperture validates and proxies to backend

## Installation

### From source

```bash
git clone https://github.com/lightninglabs/aperture.git
cd aperture
make install
```

### Using Go

```bash
go install github.com/lightninglabs/aperture/cmd/aperture@latest
```

## Quick Start (Development)

### 1. Create a minimal config

Create `aperture.yaml`:

```yaml
# Listen address for the proxy
listenaddr: "localhost:8080"

# lnd connection for creating invoices
authenticator:
  lndhost: "localhost:10009"
  tlspath: "~/.lnd/tls.cert"
  macpath: "~/.lnd/data/chain/bitcoin/regtest/admin.macaroon"

# Backend to proxy to
backend: "http://localhost:3000"

# Services that require payment
services:
  - name: "api"
    tier: "default"
    hostregexp: ".*"
    pathregexp: "/api/.*"
    price: 100  # satoshis per request

# Service tiers (pricing)
tiers:
  default:
    # No capabilities required
```

### 2. Start a simple backend

For testing, run any HTTP server:

```bash
# Python
python3 -m http.server 3000

# Or Node.js
npx http-server -p 3000

# Or Go
go run -mod=mod github.com/m3ng9i/ran@latest -p 3000
```

### 3. Start Aperture

```bash
aperture --config aperture.yaml
```

### 4. Test with lnget

```bash
# This should trigger payment flow
lnget http://localhost:8080/api/test.txt
```

## Configuration Reference

### Full config example

```yaml
# Server settings
listenaddr: "0.0.0.0:8080"
debuglevel: "info"

# TLS for the proxy itself (optional, for production)
# tlscertpath: "/path/to/cert.pem"
# tlskeypath: "/path/to/key.pem"

# LND connection
authenticator:
  lndhost: "localhost:10009"
  tlspath: "~/.lnd/tls.cert"
  macpath: "~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon"
  network: "mainnet"  # or testnet, regtest

# Backend server to proxy to
backend: "http://localhost:3000"

# Database for tracking tokens (optional)
# dbfile: "~/.aperture/aperture.db"

# Services requiring payment
services:
  - name: "premium-api"
    tier: "premium"
    hostregexp: "api\\.example\\.com"
    pathregexp: "/v1/premium/.*"
    price: 1000

  - name: "basic-api"
    tier: "basic"
    hostregexp: "api\\.example\\.com"
    pathregexp: "/v1/basic/.*"
    price: 100

  - name: "free-endpoints"
    tier: "free"
    hostregexp: ".*"
    pathregexp: "/health|/version"
    # No price = free

# Pricing tiers
tiers:
  premium:
    # Require specific capabilities in macaroon
  basic:
    # Basic access
  free:
    # No payment required
```

### Service matching

Services are matched in order. First match wins:

- `hostregexp`: Regex against Host header
- `pathregexp`: Regex against request path
- `price`: Satoshis to charge (0 or omit for free)

### Dynamic pricing

For per-resource pricing, use path captures:

```yaml
services:
  - name: "per-file"
    pathregexp: "/files/(?P<size>small|large)/.*"
    price: 100  # base price
    # Aperture can use caveats for dynamic pricing
```

## Testing Scenarios

### Test basic payment flow

```bash
# First request - should return 402
curl -v http://localhost:8080/api/data.json
# Look for: HTTP/1.1 402 Payment Required
# And: WWW-Authenticate: L402 macaroon="...", invoice="..."

# With lnget - automatic payment
lnget http://localhost:8080/api/data.json
```

### Test token reuse

```bash
# First request pays
lnget http://localhost:8080/api/file1.json

# Second request reuses token (no payment)
lnget http://localhost:8080/api/file2.json

# Verify no new payment
lnget tokens show localhost
```

### Test max-cost limit

```bash
# Configure aperture with high price (10000 sats)
# Then test with low limit
lnget --max-cost 100 http://localhost:8080/api/expensive.json
# Should fail with exit code 2
```

### Test --no-pay flag

```bash
# See the 402 response without paying
lnget --no-pay http://localhost:8080/api/data.json
```

## Docker Setup

### docker-compose.yml

```yaml
version: "3.8"

services:
  aperture:
    image: lightninglabs/aperture:latest
    ports:
      - "8080:8080"
    volumes:
      - ./aperture.yaml:/etc/aperture/aperture.yaml
      - ~/.lnd:/root/.lnd:ro
    command: ["--config", "/etc/aperture/aperture.yaml"]

  backend:
    image: nginx:alpine
    volumes:
      - ./static:/usr/share/nginx/html:ro
```

### Run

```bash
docker-compose up -d
lnget http://localhost:8080/index.html
```

## Regtest End-to-End Setup

Complete local setup for testing:

### 1. Start Bitcoin (regtest)

```bash
bitcoind -regtest -daemon -rpcuser=user -rpcpassword=pass
```

### 2. Start lnd

```bash
lnd --bitcoin.active --bitcoin.regtest --bitcoin.node=bitcoind \
    --bitcoind.rpcuser=user --bitcoind.rpcpass=pass
```

### 3. Initialize lnd wallet

```bash
lncli create
# Fund it
bitcoin-cli -regtest generatetoaddress 101 $(lncli newaddress p2wkh | jq -r .address)
```

### 4. Start second lnd (for receiving payments)

```bash
# In another terminal, different ports
lnd --bitcoin.active --bitcoin.regtest --bitcoin.node=bitcoind \
    --bitcoind.rpcuser=user --bitcoind.rpcpass=pass \
    --lnddir=~/.lnd2 --rpclisten=localhost:10010 --listen=localhost:9736
```

### 5. Connect and open channel

```bash
# Get node2 pubkey
LND2_PUBKEY=$(lncli --rpcserver=localhost:10010 getinfo | jq -r .identity_pubkey)

# Connect
lncli connect ${LND2_PUBKEY}@localhost:9736

# Open channel (1M sats)
lncli openchannel ${LND2_PUBKEY} 1000000

# Mine blocks to confirm
bitcoin-cli -regtest generatetoaddress 6 $(lncli newaddress p2wkh | jq -r .address)
```

### 6. Configure Aperture to use lnd2 (for invoices)

```yaml
authenticator:
  lndhost: "localhost:10010"
  tlspath: "~/.lnd2/tls.cert"
  macpath: "~/.lnd2/data/chain/bitcoin/regtest/admin.macaroon"
```

### 7. Configure lnget to use lnd1 (for payments)

```yaml
ln:
  lnd:
    host: localhost:10009
    tls_cert: ~/.lnd/tls.cert
    macaroon: ~/.lnd/data/chain/bitcoin/regtest/admin.macaroon
```

### 8. Test

```bash
lnget http://localhost:8080/api/test.txt
# Payment flows from lnd1 → lnd2 via the channel
```

## Troubleshooting

### "invoice not found"

Aperture creates invoices via lnd. Check:
```bash
# Verify aperture's lnd connection
lncli --rpcserver=localhost:10010 getinfo
```

### "no route found"

Payment can't reach the destination:
- Check channels are open and confirmed
- Verify sufficient outbound capacity
- For regtest, ensure both nodes are connected

### "macaroon validation failed"

Token was rejected:
- Token may have expired
- Clear tokens and retry: `lnget tokens clear`

## Production Considerations

1. **Use TLS**: Configure `tlscertpath` and `tlskeypath`
2. **Rate limiting**: Consider additional rate limiting in front of Aperture
3. **Monitoring**: Aperture exposes Prometheus metrics
4. **Database**: Use persistent database for token tracking across restarts

## Next Steps

- [Workflow Examples](workflows.md) - Common lnget usage patterns
- [Agent Guide](agents.md) - Architecture for developers
