#!/usr/bin/env bash
# Install lnd and lncli from source with required build tags.
#
# Usage:
#   install.sh              # Install latest release
#   install.sh --version v0.18.0-beta  # Specific version
#
# Prerequisites: Go 1.21+

set -e

VERSION=""
BUILD_TAGS="signrpc walletrpc chainrpc invoicesrpc routerrpc peersrpc kvdb_sqlite neutrinorpc"

# Parse arguments.
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="@$2"
            shift 2
            ;;
        --tags)
            BUILD_TAGS="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: install.sh [--version VERSION] [--tags TAGS]"
            echo ""
            echo "Install lnd and lncli from source."
            echo ""
            echo "Options:"
            echo "  --version VERSION  Go module version (e.g., v0.18.0-beta)"
            echo "  --tags TAGS        Build tags (default: signrpc walletrpc ...)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

echo "=== Installing lnd ==="
echo ""

# Verify Go is installed.
if ! command -v go &>/dev/null; then
    echo "Error: Go is not installed." >&2
    echo "Install Go from https://go.dev/dl/" >&2
    exit 1
fi

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1)
echo "Go version: $GO_VERSION"
echo "Build tags: $BUILD_TAGS"
echo ""

# Install lnd.
echo "Installing lnd..."
go install -tags "$BUILD_TAGS" "github.com/lightningnetwork/lnd/cmd/lnd${VERSION}"
echo "Done."

# Install lncli.
echo "Installing lncli..."
go install -tags "$BUILD_TAGS" "github.com/lightningnetwork/lnd/cmd/lncli${VERSION}"
echo "Done."
echo ""

# Verify installation.
if command -v lnd &>/dev/null; then
    echo "lnd installed: $(which lnd)"
    lnd --version 2>/dev/null || true
else
    echo "Warning: lnd not found on PATH." >&2
    echo "Ensure \$GOPATH/bin is in your PATH." >&2
    echo "  export PATH=\$PATH:\$(go env GOPATH)/bin" >&2
fi

if command -v lncli &>/dev/null; then
    echo "lncli installed: $(which lncli)"
else
    echo "Warning: lncli not found on PATH." >&2
fi

echo ""
echo "Installation complete."
echo ""
echo "Next steps:"
echo "  1. Create wallet: skills/lnd/scripts/create-wallet.sh"
echo "  2. Start lnd:     skills/lnd/scripts/start-lnd.sh"
