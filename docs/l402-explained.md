# Understanding L402

L402 is a protocol for API monetization using the Lightning Network. This document explains what it is, why it exists, and how it works.

## The Problem L402 Solves

Traditional API monetization has friction:

1. **Account creation**: Users must sign up, verify email, add payment method
2. **Subscription overhead**: Pay monthly even if you use the API once
3. **Minimum commitments**: Many APIs require prepaid credits
4. **Privacy concerns**: Payment details tied to identity

For programmatic access—especially by AI agents—this friction is a blocker. An agent can't create accounts or manage subscriptions.

## The L402 Solution

L402 enables **pay-per-request** APIs with no accounts:

1. Client requests a resource
2. Server returns HTTP 402 "Payment Required" with a Lightning invoice
3. Client pays the invoice (instant, global, pseudonymous)
4. Client retries with proof of payment
5. Server delivers the resource

No signup. No subscription. Just pay and use.

## How It Works

### The Challenge-Response Flow

```
Client                              Server
  |                                    |
  |  GET /api/data                     |
  |----------------------------------->|
  |                                    |
  |  402 Payment Required              |
  |  WWW-Authenticate: L402            |
  |    macaroon="abc...",              |
  |    invoice="lnbc..."               |
  |<-----------------------------------|
  |                                    |
  |  [Pay invoice via Lightning]       |
  |                                    |
  |  GET /api/data                     |
  |  Authorization: L402 abc...:xyz... |
  |----------------------------------->|
  |                                    |
  |  200 OK                            |
  |  {data}                            |
  |<-----------------------------------|
```

### The Components

**Macaroon**: A bearer credential with embedded caveats (restrictions). Think of it as a "ticket" that can have conditions attached.

**Invoice**: A Lightning Network payment request (BOLT11 format). Contains:
- Amount to pay
- Payment hash (unique identifier)
- Expiry time
- Destination node

**Preimage**: The secret revealed when an invoice is paid. Proves payment was made.

**L402 Token**: The combination of macaroon + preimage that authorizes access.

### The WWW-Authenticate Header

When a server requires payment, it returns:

```
HTTP/1.1 402 Payment Required
WWW-Authenticate: L402 macaroon="<base64>", invoice="<bolt11>"
```

The macaroon is tied to the invoice's payment hash. When the client pays the invoice, they receive the preimage. The macaroon + preimage together form the L402 token.

### The Authorization Header

After payment, the client includes:

```
Authorization: L402 <macaroon>:<preimage>
```

Both are base64-encoded. The server verifies:
1. The macaroon is valid (signature, caveats)
2. The preimage matches the payment hash embedded in the macaroon
3. Any caveats (restrictions) are satisfied

## Macaroons Explained

Macaroons are like cookies with superpowers:

### Caveats

Restrictions can be added to a macaroon without the server's involvement:

```
Original macaroon:
  - Signed by server
  - Contains payment hash

Attenuated macaroon:
  - Original + "expires: 2024-01-01"
  - Original + "ip: 192.168.1.1"
  - Still verifiable by server
```

This enables:
- Time-limited access
- IP restrictions
- Usage caps
- Delegation to third parties

### Delegation

Alice pays for a macaroon, adds a caveat ("only GET requests"), and gives it to Bob. Bob can use it within those restrictions. Alice doesn't need to trust Bob with full access.

## Why Lightning?

Lightning Network payments are ideal for API monetization:

| Feature | Traditional Payment | Lightning |
|---------|-------------------|-----------|
| Settlement time | Days | Seconds |
| Minimum viable payment | ~$0.30 (card fees) | <$0.01 |
| Account required | Yes | No |
| Chargebacks | Yes | No |
| Global reach | Limited | Instant |
| Privacy | Poor | Good |

Micropayments become viable. A 10-sat (~$0.004) API call makes economic sense.

## L402 vs. Alternatives

### vs. API Keys

- API keys require account creation
- L402 tokens are self-authenticating
- API keys can't be safely delegated
- L402 supports attenuation via caveats

### vs. OAuth

- OAuth requires registration and consent flows
- L402 is pay-and-use, no accounts
- OAuth is identity-centric
- L402 is payment-centric

### vs. Prepaid Credits

- Credits require upfront payment
- L402 is pay-as-you-go
- Credits lock up capital
- L402 payments are instant

## Implementation in lnget

lnget handles the L402 flow transparently:

```
1. Request     → lnget makes HTTP request
2. 402 Check   → Detects L402 challenge in response
3. Parse       → Extracts macaroon and invoice
4. Validate    → Checks invoice amount against --max-cost
5. Pay         → Sends payment via Lightning backend
6. Cache       → Stores token for future requests
7. Retry       → Repeats request with Authorization header
```

### Token Caching

lnget caches tokens per-domain at `~/.lnget/tokens/<domain>/token.json`:

```json
{
  "macaroon": "base64...",
  "preimage": "hex...",
  "payment_hash": "hex...",
  "amount_msat": 100000,
  "created_at": "2024-01-15T10:30:00Z"
}
```

Subsequent requests to the same domain reuse the token—no additional payment.

### Cost Controls

lnget protects against unexpected payments:

```bash
# Only pay invoices under 1000 sats (default)
lnget https://api.example.com/data.json

# Increase limit for expensive resources
lnget --max-cost 10000 https://api.example.com/premium.json

# Preview price without paying
lnget --no-pay https://api.example.com/data.json
```

## Security Considerations

### For Clients

- **Set cost limits**: Use `--max-cost` to prevent unexpected charges
- **Verify domains**: Tokens are domain-scoped, but verify you trust the endpoint
- **Protect tokens**: Cached tokens are bearer credentials

### For Servers (Aperture)

- **Short invoice expiry**: Limit window for payment
- **Macaroon caveats**: Add restrictions (time, IP, usage count)
- **Rate limiting**: L402 prevents spam but consider additional limits

## The L402 Ecosystem

### Aperture

Lightning Labs' reference L402 proxy. Sits in front of your API and handles:
- Challenge generation
- Invoice creation
- Token verification
- Request proxying

### lnget

This tool. Handles L402 transparently for command-line HTTP requests.

### Libraries

- `aperture/l402` (Go) - L402 token handling
- `lsat-js` (JavaScript) - Browser/Node.js support

## Further Reading

- [L402 Protocol Specification](https://github.com/lightninglabs/L402)
- [Macaroons Paper](https://research.google/pubs/pub41892/)
- [BOLT11 Invoice Format](https://github.com/lightning/bolts/blob/master/11-payment-encoding.md)
- [Aperture Documentation](https://github.com/lightninglabs/aperture)
