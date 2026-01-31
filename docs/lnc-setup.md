# Lightning Node Connect (LNC) Setup

Lightning Node Connect enables secure remote access to your Lightning node without exposing macaroons or TLS certificates directly. This guide covers setting up lnget with LNC.

## What is LNC?

LNC creates an encrypted tunnel between lnget and your lnd node through a relay server:

```
┌─────────┐         ┌─────────────┐         ┌─────────┐
│  lnget  │◀───────▶│ LNC Relay   │◀───────▶│   lnd   │
│ (local) │  noise  │ (Lightning  │  noise  │(remote) │
│         │  proto  │  Terminal)  │  proto  │         │
└─────────┘         └─────────────┘         └─────────┘
```

**Benefits:**

- No macaroon/TLS files to manage
- Works through firewalls and NAT
- Revocable sessions
- End-to-end encryption

## Prerequisites

- lnd v0.20.0 or later with LNC enabled
- Lightning Terminal (LiT) or standalone lnd with litd
- Network access to Lightning Terminal relay

## Setup Options

### Option 1: Using Lightning Terminal (LiT)

If you run [Lightning Terminal](https://github.com/lightninglabs/lightning-terminal):

1. **Open Lightning Terminal UI** at https://localhost:8443

2. **Create a session**:
   - Click "Lightning Node Connect"
   - Click "Create New Session"
   - Set permissions (read + send for lnget)
   - Copy the pairing phrase

3. **Pair with lnget**:
   ```bash
   lnget ln lnc pair "your-pairing-phrase-here"
   ```

4. **Configure lnget to use LNC**:
   ```yaml
   ln:
     mode: lnc
   ```

5. **Verify connection**:
   ```bash
   lnget ln status
   ```

### Option 2: Using lnd + litd

If running lnd separately with litd:

1. **Start litd** alongside lnd:
   ```bash
   litd --lnd-mode=remote \
        --remote.lnd.rpcserver=localhost:10009 \
        --remote.lnd.macaroonpath=~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon \
        --remote.lnd.tlscertpath=~/.lnd/tls.cert
   ```

2. **Create session via litcli**:
   ```bash
   litcli sessions add --label="lnget" --type=admin
   # Copy the pairing phrase
   ```

3. **Pair with lnget**:
   ```bash
   lnget ln lnc pair "pairing-phrase"
   ```

## Pairing Process

When you run `lnget ln lnc pair`:

1. lnget contacts the LNC relay
2. Performs Noise protocol handshake
3. Establishes encrypted channel
4. Stores session credentials locally at `~/.lnget/lnc/`

```bash
$ lnget ln lnc pair "flexible hammer potato..."
Connecting to Lightning Terminal...
Pairing successful!
Session stored: ~/.lnget/lnc/session.json

$ lnget ln status
Backend: lnc
Status: connected
Pubkey: 03abc123...
```

## Session Management

### List sessions

```bash
lnget ln lnc sessions
```

Output:
```
Sessions:
  ID: abc123
  Label: lnget
  State: active
  Created: 2024-01-15
  Expires: 2024-02-15
```

### Revoke a session

From lnget (if you have access):
```bash
lnget ln lnc revoke abc123
```

From Lightning Terminal UI:
- Navigate to LNC sessions
- Click "Revoke" on the session

**Important**: Revoking immediately invalidates the session. lnget will need a new pairing phrase.

### Session expiry

Sessions can have expiration times. When expired:
```
Error: LNC session expired, please re-pair
```

Create a new session and pair again.

## Security Considerations

### What LNC protects

- **No credential exposure**: Macaroons/TLS certs stay on the server
- **Revocable access**: Sessions can be invalidated remotely
- **Encrypted tunnel**: End-to-end encryption via Noise protocol

### What to be aware of

- **Relay trust**: The relay (Lightning Terminal) sees metadata but not content
- **Session file security**: `~/.lnget/lnc/session.json` grants access—protect it
- **Permission scope**: Sessions have permissions; create minimal-permission sessions

### Recommended permissions for lnget

When creating a session, grant only:
- **Read**: For `getinfo`, balance checks
- **Send**: For paying invoices

Don't grant:
- **Admin**: Full control (unnecessary for lnget)
- **Receive**: Creating invoices (lnget doesn't need this)

## Configuration

### Full LNC config

```yaml
ln:
  mode: lnc
  lnc:
    # Session file location (default: ~/.lnget/lnc/session.json)
    session_path: ~/.lnget/lnc/session.json

    # Relay server (default: Lightning Terminal)
    # relay: wss://terminal.lightning.engineering

    # Connection timeout
    timeout: 30s
```

### Environment variables

```bash
export LNGET_LN_MODE=lnc
export LNGET_LN_LNC_SESSION_PATH=~/.lnget/lnc/session.json
```

## Troubleshooting

### "session not found"

```
Error: LNC session not found at ~/.lnget/lnc/session.json
```

**Fix**: Pair first:
```bash
lnget ln lnc pair "your-pairing-phrase"
```

### "session expired"

```
Error: LNC session has expired
```

**Fix**: Create new session in Lightning Terminal and re-pair.

### "connection refused" to relay

```
Error: failed to connect to LNC relay
```

**Causes**:
- Network issues
- Relay server down
- Firewall blocking WebSocket

**Fix**: Check network, try again later, verify firewall allows wss:// connections.

### "authentication failed"

```
Error: LNC authentication failed
```

**Causes**:
- Session revoked on server
- Session file corrupted

**Fix**:
```bash
# Remove local session
rm ~/.lnget/lnc/session.json
# Re-pair with new phrase
lnget ln lnc pair "new-pairing-phrase"
```

### Slow connection

LNC adds latency vs direct gRPC:

```
Direct lnd:  ~10ms per call
Via LNC:     ~100-300ms per call
```

This is normal due to relay routing. For latency-sensitive use cases, consider direct lnd connection.

## Use Cases

### Remote node access

Access your home node from anywhere:
```bash
# On laptop, paired with home node
lnget https://api.example.com/data.json
# Payment routed through home node
```

### Multi-device setup

Same node, multiple devices:
1. Create separate sessions for each device
2. Pair each device
3. Revoke sessions as needed

### Temporary access

Grant time-limited access:
1. Create session with expiry
2. Share pairing phrase
3. Session auto-expires

## Comparison with Direct lnd

| Aspect | Direct lnd | LNC |
|--------|-----------|-----|
| Latency | Low | Higher |
| Setup | Manage credentials | Pairing phrase |
| Security | Credential exposure | No exposure |
| Revocation | Delete macaroon | Remote revoke |
| Firewall | Need port forward | Works through NAT |

**Use LNC when**:
- Accessing node remotely
- Don't want to manage credentials
- Need revocable access

**Use direct lnd when**:
- Low latency required
- Same machine as lnd
- Production server setup

## Next Steps

- [Setting up lnd](setup-lnd.md) - Direct lnd connection
- [Neutrino setup](neutrino-setup.md) - Embedded wallet
- [Workflows](workflows.md) - Usage examples
