#!/usr/bin/env bash
# Wrapper for lncli with auto-detected paths.
#
# Usage:
#   lncli.sh getinfo
#   lncli.sh walletbalance
#   lncli.sh --network testnet getinfo
#   lncli.sh openchannel --node_key=<pubkey> --local_amt=1000000

set -e

LND_DIR="${LND_DIR:-$HOME/.lnd}"
NETWORK="${NETWORK:-mainnet}"
LNCLI_ARGS=()

# Parse our arguments (pass everything else to lncli).
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
        -h|--help)
            echo "Usage: lncli.sh [--network NET] [--lnddir DIR] <command> [args]"
            echo ""
            echo "Wrapper for lncli with auto-detected paths."
            echo ""
            echo "Options:"
            echo "  --network NETWORK  Bitcoin network (default: mainnet)"
            echo "  --lnddir DIR       lnd data directory (default: ~/.lnd)"
            echo ""
            echo "All other arguments are passed directly to lncli."
            exit 0
            ;;
        *)
            LNCLI_ARGS+=("$1")
            shift
            ;;
    esac
done

if [ ${#LNCLI_ARGS[@]} -eq 0 ]; then
    echo "Error: No lncli command specified." >&2
    echo "Usage: lncli.sh <command> [args]" >&2
    exit 1
fi

# Verify lncli is installed.
if ! command -v lncli &>/dev/null; then
    echo "Error: lncli not found. Run install.sh first." >&2
    exit 1
fi

exec lncli --network="$NETWORK" --lnddir="$LND_DIR" "${LNCLI_ARGS[@]}"
