# LND Wallet Security Guide

## Current Model: Passphrase on Disk

The default setup stores wallet credentials as files on disk for agent
automation convenience:

| File | Contents | Permissions |
|------|----------|-------------|
| `~/.lnget/lnd/wallet-password.txt` | Wallet unlock passphrase | 0600 |
| `~/.lnget/lnd/seed.txt` | 24-word BIP39 mnemonic | 0600 |

**This is suitable for:**
- Testnet and signet development
- Small mainnet amounts for micropayments
- Agent automation where convenience is prioritized

**Risks:**
- Any process running as the same user can read the files
- Disk compromise exposes both passphrase and seed
- No protection against malicious software on the same machine

## Wallet Passphrase

The wallet passphrase encrypts the wallet database on disk. Without it, the
wallet cannot be unlocked and funds cannot be spent.

**Auto-unlock:** The default lnd config includes `wallet-unlock-password-file`
which reads the passphrase on startup. This means the node is operational
immediately after a restart without manual intervention.

**Manual unlock:** Remove `wallet-unlock-password-file` from `lnd.conf` to
require manual unlock via `lncli unlock` or the REST API after each restart.

## Seed Mnemonic

The 24-word seed is the master secret from which all keys are derived. With the
seed, the entire wallet can be reconstructed on any lnd instance.

**Current storage:** Plain text file at `~/.lnget/lnd/seed.txt` with mode 0600.

**Recommended improvements (in order of increasing security):**

1. **Encrypted file:** Encrypt the seed file with a separate passphrase using
   GPG or age. The agent would need the encryption passphrase only during
   wallet recovery.

2. **OS keychain:** Store the seed in the operating system's keychain (macOS
   Keychain, Linux Secret Service). Requires keychain unlock but survives
   disk inspection.

3. **Remote signer:** Use lnd's remote signer mode where the seed and signing
   keys live on a separate machine. The agent's lnd instance never has access
   to the seed material. This is the production-grade solution.

## Remote Signer Mode (Future)

lnd supports a remote signing architecture where:

1. **Watch-only node** (the agent's node) handles networking, channel state,
   and routing but cannot sign transactions.
2. **Signing node** (separate, secured machine) holds the seed and performs
   all cryptographic signing operations.

Setup overview:

```
Agent Node (watch-only)  <--gRPC-->  Signing Node (keys)
  - Runs neutrino                      - Holds seed
  - Manages channels                   - Signs commitments
  - Routes payments                    - Signs on-chain txs
  - No key material                    - Minimal attack surface
```

This architecture means compromising the agent's machine does not expose the
wallet seed or signing keys.

## Macaroon Security

lnd uses macaroons for API authentication. The key macaroons:

| Macaroon | Capabilities |
|----------|-------------|
| `admin.macaroon` | Full access (read, write, generate invoices, send payments) |
| `readonly.macaroon` | Read-only access (getinfo, balances, list operations) |
| `invoice.macaroon` | Create and manage invoices only |

**For agents that only need to pay invoices** (e.g., lnget), a custom
macaroon with restricted permissions is recommended:

```bash
lncli bakemacaroon uri:/lnrpc.Lightning/SendPaymentSync \
    uri:/lnrpc.Lightning/DecodePayReq \
    uri:/lnrpc.Lightning/GetInfo \
    --save_to=~/.lnd/data/chain/bitcoin/mainnet/pay-only.macaroon
```

## Checklist for Production Use

- [ ] Use testnet/signet for development, mainnet only for production
- [ ] Set file permissions: `chmod 600` on all credential files
- [ ] Consider encrypting seed file at rest
- [ ] Use restricted macaroons (not admin) where possible
- [ ] Monitor wallet balance for unexpected changes
- [ ] Keep lnd updated to latest stable release
- [ ] Plan migration to remote signer for significant funds
- [ ] Backup seed in a separate, secure location
