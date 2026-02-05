#!/usr/bin/env bash
# Start lnd with neutrino backend and SQLite storage.
#
# Usage:
#   start-lnd.sh                        # Default (mainnet, background)
#   start-lnd.sh --network testnet      # Testnet
#   start-lnd.sh --foreground           # Run in foreground
#   start-lnd.sh --extra-args "--debuglevel=trace"

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LNGET_LND_DIR="${LNGET_LND_DIR:-$HOME/.lnget/lnd}"
LND_DIR="${LND_DIR:-$HOME/.lnd}"
NETWORK="mainnet"
FOREGROUND=false
EXTRA_ARGS=""
CONF_FILE="$LNGET_LND_DIR/lnd.conf"

# Parse arguments.
while [[ $# -gt 0 ]]; do
    case $1 in
        --network)
            NETWORK="$2"
            shift 2
            ;;
        --lnddir)
            LND_DIR="$2"
            shift 2
            ;;
        --foreground)
            FOREGROUND=true
            shift
            ;;
        --extra-args)
            EXTRA_ARGS="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: start-lnd.sh [options]"
            echo ""
            echo "Start lnd with neutrino backend."
            echo ""
            echo "Options:"
            echo "  --network NETWORK    Bitcoin network (default: mainnet)"
            echo "  --lnddir DIR         lnd data directory (default: ~/.lnd)"
            echo "  --foreground         Run in foreground (default: background)"
            echo "  --extra-args ARGS    Additional lnd arguments"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Verify lnd is installed.
if ! command -v lnd &>/dev/null; then
    echo "Error: lnd not found. Run install.sh first." >&2
    exit 1
fi

# Check if lnd is already running.
if pgrep -x lnd &>/dev/null; then
    echo "lnd is already running (PID: $(pgrep -x lnd))."
    echo "Use stop-lnd.sh to stop it first."
    exit 1
fi

# Create config directory if needed.
mkdir -p "$LNGET_LND_DIR"

# Copy config template if no config exists.
if [ ! -f "$CONF_FILE" ]; then
    TEMPLATE="$SCRIPT_DIR/../templates/lnd.conf.template"
    if [ -f "$TEMPLATE" ]; then
        echo "Creating config from template..."
        # Replace network placeholder in template.
        sed "s/bitcoin\.mainnet=true/bitcoin.$NETWORK=true/g" "$TEMPLATE" > "$CONF_FILE"

        # Replace password file path.
        sed -i.bak "s|wallet-unlock-password-file=.*|wallet-unlock-password-file=$LNGET_LND_DIR/wallet-password.txt|g" "$CONF_FILE"
        rm -f "$CONF_FILE.bak"
    else
        echo "Warning: No config template found. lnd will use defaults." >&2
    fi
fi

echo "=== Starting lnd ==="
echo "Network:  $NETWORK"
echo "Data dir: $LND_DIR"
echo "Config:   $CONF_FILE"
echo ""

LOG_FILE="$LNGET_LND_DIR/lnd-start.log"

if [ "$FOREGROUND" = true ]; then
    exec lnd \
        --lnddir="$LND_DIR" \
        --configfile="$CONF_FILE" \
        $EXTRA_ARGS
else
    nohup lnd \
        --lnddir="$LND_DIR" \
        --configfile="$CONF_FILE" \
        $EXTRA_ARGS \
        > "$LOG_FILE" 2>&1 &
    LND_PID=$!
    echo "lnd started in background (PID: $LND_PID)"
    echo "Log file: $LOG_FILE"
    echo ""

    # Wait briefly and verify it's running.
    sleep 2
    if kill -0 "$LND_PID" 2>/dev/null; then
        echo "lnd is running."
    else
        echo "Error: lnd exited immediately. Check $LOG_FILE" >&2
        tail -20 "$LOG_FILE" 2>/dev/null
        exit 1
    fi

    echo ""
    echo "Next steps:"
    echo "  # Check status"
    echo "  skills/lnd/scripts/lncli.sh getinfo"
    echo ""
    echo "  # If wallet not yet created"
    echo "  skills/lnd/scripts/create-wallet.sh"
fi
