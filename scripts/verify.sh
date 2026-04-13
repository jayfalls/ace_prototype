#!/usr/bin/env bash
set -e

# ACE Binary Verification Script
# Standalone script for verifying SHA256 checksums of downloaded binaries

set -e

usage() {
    echo "Usage: $0 <path-to-ace-binary>"
    echo "  Verifies the SHA256 checksum of an ace binary against the official checksums.txt"
    exit 1
}

if [ $# -lt 1 ]; then
    usage
fi

BINARY_PATH="$1"
REPO="ace-org/ace"

# Check if binary exists
if [ ! -f "${BINARY_PATH}" ]; then
    echo "Error: file not found: ${BINARY_PATH}" >&2
    exit 1
fi

# Detect OS and architecture for checksum lookup
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux) ;;
    darwin) ;;
    *)
        echo "Error: unsupported operating system: ${OS}. ACE supports linux and darwin." >&2
        exit 1
        ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: ${ARCH}. ACE supports amd64 and arm64." >&2
        exit 1
        ;;
esac

# Fetch latest release tag
echo "Fetching latest release info..."
LATEST_TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": "v?([^"]+)".*/\1/')
if [ -z "$LATEST_TAG" ]; then
    echo "Error: unable to fetch latest release from GitHub." >&2
    exit 1
fi

# Download checksums
echo "Downloading checksums.txt..."
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/checksums.txt"
TMP_CHECKSUMS=$(mktemp)
trap "rm -f ${TMP_CHECKSUMS}" EXIT

if ! curl -fsSL "${CHECKSUMS_URL}" -o "${TMP_CHECKSUMS}"; then
    echo "Error: failed to download checksums." >&2
    exit 1
fi

# Find expected hash
EXPECTED_HASH=$(grep "ace_${OS}_${ARCH}$" "${TMP_CHECKSUMS}" | awk '{print $1}')
if [ -z "$EXPECTED_HASH" ]; then
    echo "Error: checksum not found for ace_${OS}_${ARCH} in checksums.txt" >&2
    exit 1
fi

# Calculate actual hash
ACTUAL_HASH=$(sha256sum "${BINARY_PATH}" | awk '{print $1}')

# Compare
echo "Expected: ${EXPECTED_HASH}"
echo "Actual:   ${ACTUAL_HASH}"

if [ "$EXPECTED_HASH" = "$ACTUAL_HASH" ]; then
    echo ""
    echo "Verification successful: SHA256 checksum matches."
    exit 0
else
    echo ""
    echo "Error: checksum mismatch! The binary may have been tampered with." >&2
    exit 1
fi