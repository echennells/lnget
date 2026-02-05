#!/usr/bin/env bash
# Create an encrypted lnd wallet with secure credential storage.
#
# Usage:
#   create-wallet.sh                         # Auto-generate passphrase
#   create-wallet.sh --password "mypass"     # Custom passphrase
#   create-wallet.sh --network testnet       # Testnet wallet
#   create-wallet.sh --recover --seed-file ~/.lnget/lnd/seed.txt  # Recover
#
# Stores credentials at:
#   ~/.lnget/lnd/wallet-password.txt  (mode 0600)
#   ~/.lnget/lnd/seed.txt             (mode 0600)

set -e

LNGET_LND_DIR="${LNGET_LND_DIR:-$HOME/.lnget/lnd}"
LND_DIR="${LND_DIR:-$HOME/.lnd}"
NETWORK="mainnet"
PASSWORD=""
RECOVER=false
SEED_FILE=""
REST_PORT=8080

# Parse arguments.
while [[ $# -gt 0 ]]; do
    case $1 in
        --password)
            PASSWORD="$2"
            shift 2
            ;;
        --network)
            NETWORK="$2"
            shift 2
            ;;
        --lnddir)
            LND_DIR="$2"
            shift 2
            ;;
        --recover)
            RECOVER=true
            shift
            ;;
        --seed-file)
            SEED_FILE="$2"
            shift 2
            ;;
        --rest-port)
            REST_PORT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: create-wallet.sh [options]"
            echo ""
            echo "Create an encrypted lnd wallet."
            echo ""
            echo "Options:"
            echo "  --password PASS     Wallet passphrase (auto-generated if omitted)"
            echo "  --network NETWORK   Bitcoin network (default: mainnet)"
            echo "  --lnddir DIR        lnd data directory (default: ~/.lnd)"
            echo "  --recover           Recover wallet from existing seed"
            echo "  --seed-file FILE    Path to seed file for recovery"
            echo "  --rest-port PORT    lnd REST port (default: 8080)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

echo "=== LND Wallet Setup ==="
echo ""
echo "Network:    $NETWORK"
echo "lnd dir:    $LND_DIR"
echo "Creds dir:  $LNGET_LND_DIR"
echo ""

# Create credential storage directory with restricted permissions.
mkdir -p "$LNGET_LND_DIR"
chmod 700 "$LNGET_LND_DIR"

PASSWORD_FILE="$LNGET_LND_DIR/wallet-password.txt"
SEED_OUTPUT="$LNGET_LND_DIR/seed.txt"

# Generate or use provided passphrase.
if [ -n "$PASSWORD" ]; then
    echo "Using provided passphrase."
else
    echo "Generating secure passphrase..."
    PASSWORD=$(openssl rand -base64 32 | tr -d '/+=' | head -c 32)
fi

# Store passphrase with restricted permissions.
echo -n "$PASSWORD" > "$PASSWORD_FILE"
chmod 600 "$PASSWORD_FILE"
echo "Passphrase saved to $PASSWORD_FILE (mode 0600)"
echo ""

# Check if lnd is running; if not, we need to start it temporarily.
LND_WAS_RUNNING=true
if ! lncli --network="$NETWORK" --lnddir="$LND_DIR" getinfo &>/dev/null 2>&1; then
    LND_WAS_RUNNING=false

    # Check if wallet already exists (lnd running but locked).
    if pgrep -x lnd &>/dev/null; then
        echo "lnd is running but wallet is locked or not yet created."
    else
        echo "Starting lnd temporarily for wallet creation..."
        lnd --lnddir="$LND_DIR" \
            --bitcoin.active \
            "--bitcoin.$NETWORK" \
            --bitcoin.node=neutrino \
            --neutrino.addpeer=btcd0.lightning.computer \
            --neutrino.addpeer=mainnet1-btcd.zaphq.io \
            --db.backend=sqlite \
            "--restlisten=localhost:$REST_PORT" \
            --rpclisten=localhost:10009 &
        LND_PID=$!

        echo "Waiting for lnd to start (PID: $LND_PID)..."
        for i in {1..30}; do
            # Check if the REST endpoint is up.
            if curl -sk "https://localhost:$REST_PORT/v1/state" &>/dev/null; then
                break
            fi
            sleep 2
            echo "  Waiting... ($i/30)"
        done
        echo ""
    fi
fi

# Create or recover wallet via REST API.
if [ "$RECOVER" = true ]; then
    echo "=== Recovering Wallet ==="
    if [ -z "$SEED_FILE" ]; then
        echo "Error: --seed-file required for recovery" >&2
        exit 1
    fi
    if [ ! -f "$SEED_FILE" ]; then
        echo "Error: Seed file not found: $SEED_FILE" >&2
        exit 1
    fi

    # Read seed words from file.
    SEED_WORDS=$(cat "$SEED_FILE" | tr '\n' ' ' | xargs)
    SEED_JSON=$(echo "$SEED_WORDS" | tr ' ' '\n' | jq -R . | jq -s .)

    # Build recovery request.
    PAYLOAD=$(jq -n \
        --arg pass "$(echo -n "$PASSWORD" | base64)" \
        --argjson seed "$SEED_JSON" \
        '{wallet_password: $pass, cipher_seed_mnemonic: $seed}')

    RESPONSE=$(curl -sk -X POST \
        "https://localhost:$REST_PORT/v1/initwallet" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" 2>&1)

    echo "Wallet recovered successfully."
else
    echo "=== Creating New Wallet ==="

    # Generate seed first.
    echo "Generating wallet seed..."
    SEED_RESPONSE=$(curl -sk -X GET \
        "https://localhost:$REST_PORT/v1/genseed" 2>&1)

    # Extract mnemonic.
    MNEMONIC=$(echo "$SEED_RESPONSE" | jq -r '.cipher_seed_mnemonic[]' 2>/dev/null)
    if [ -z "$MNEMONIC" ] || [ "$MNEMONIC" = "null" ]; then
        echo "Error: Failed to generate seed." >&2
        echo "Response: $SEED_RESPONSE" >&2
        exit 1
    fi

    # Store seed with restricted permissions.
    echo "$MNEMONIC" > "$SEED_OUTPUT"
    chmod 600 "$SEED_OUTPUT"
    echo "Seed mnemonic saved to $SEED_OUTPUT (mode 0600)"
    echo ""

    # Initialize wallet with password and seed.
    SEED_JSON=$(echo "$MNEMONIC" | jq -R . | jq -s .)
    PAYLOAD=$(jq -n \
        --arg pass "$(echo -n "$PASSWORD" | base64)" \
        --argjson seed "$SEED_JSON" \
        '{wallet_password: $pass, cipher_seed_mnemonic: $seed}')

    RESPONSE=$(curl -sk -X POST \
        "https://localhost:$REST_PORT/v1/initwallet" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" 2>&1)

    # Check for errors.
    ERROR=$(echo "$RESPONSE" | jq -r '.message // empty' 2>/dev/null)
    if [ -n "$ERROR" ]; then
        echo "Error creating wallet: $ERROR" >&2
        exit 1
    fi

    echo "Wallet created successfully!"
fi

echo ""
echo "=== Credential Locations ==="
echo "  Passphrase: $PASSWORD_FILE"
echo "  Seed:       $SEED_OUTPUT"
echo ""
echo "IMPORTANT: Both files are stored with restricted permissions (0600)."
echo "The seed mnemonic is your wallet backup. Keep it safe."
echo "For production use, consider migrating to remote signer mode."
echo ""
echo "Next steps:"
echo "  1. Start lnd: skills/lnd/scripts/start-lnd.sh"
echo "  2. Fund wallet: skills/lnd/scripts/lncli.sh newaddress p2tr"
