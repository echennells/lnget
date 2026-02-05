#!/usr/bin/env bash
# Unlock lnd wallet using stored passphrase via REST API.
#
# Usage:
#   unlock-wallet.sh                           # Default paths
#   unlock-wallet.sh --rest-port 8080          # Custom REST port
#   unlock-wallet.sh --password-file /path     # Custom password file

set -e

LNGET_LND_DIR="${LNGET_LND_DIR:-$HOME/.lnget/lnd}"
PASSWORD_FILE="$LNGET_LND_DIR/wallet-password.txt"
REST_PORT=8080

# Parse arguments.
while [[ $# -gt 0 ]]; do
    case $1 in
        --password-file)
            PASSWORD_FILE="$2"
            shift 2
            ;;
        --rest-port)
            REST_PORT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: unlock-wallet.sh [options]"
            echo ""
            echo "Unlock lnd wallet using stored passphrase."
            echo ""
            echo "Options:"
            echo "  --password-file FILE  Path to password file"
            echo "  --rest-port PORT      lnd REST port (default: 8080)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Verify password file exists.
if [ ! -f "$PASSWORD_FILE" ]; then
    echo "Error: Password file not found: $PASSWORD_FILE" >&2
    echo "Run create-wallet.sh first to create the wallet." >&2
    exit 1
fi

PASSWORD=$(cat "$PASSWORD_FILE")

echo "Unlocking lnd wallet via REST API (port $REST_PORT)..."

# Call the unlock endpoint.
PAYLOAD=$(jq -n --arg pass "$(echo -n "$PASSWORD" | base64)" \
    '{wallet_password: $pass}')

RESPONSE=$(curl -sk -X POST \
    "https://localhost:$REST_PORT/v1/unlockwallet" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" 2>&1)

# Check for errors.
ERROR=$(echo "$RESPONSE" | jq -r '.message // empty' 2>/dev/null)
if [ -n "$ERROR" ]; then
    if echo "$ERROR" | grep -q "already unlocked"; then
        echo "Wallet is already unlocked."
        exit 0
    fi
    echo "Error unlocking wallet: $ERROR" >&2
    exit 1
fi

echo "Wallet unlocked successfully."
