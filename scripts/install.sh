#!/usr/bin/env bash
set -e

# ACE Installation Script
# Follows Anchore/Syft pattern for installation scripts

set -e

REPO="ace-org/ace"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="ace"

usage() {
    echo "Usage: $0 [-b install_dir]"
    echo "  -b    Install to custom directory (default: ~/.local/bin)"
    exit 1
}

# Parse flags
while getopts "b:h" flag; do
    case "$flag" in
        b) INSTALL_DIR="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux) ;;
    darwin) ;;
    *)
        echo "Error: unsupported operating system: ${OS}. ACE supports linux and darwin." >&2
        exit 1
        ;;
esac

# Detect architecture
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
echo "Fetching latest release..."
LATEST_TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": "v?([^"]+)".*/\1/')
if [ -z "$LATEST_TAG" ]; then
    echo "Error: unable to fetch latest release from GitHub." >&2
    exit 1
fi

echo "Latest release: ${LATEST_TAG}"

# Construct download URLs
BINARY_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/ace_${OS}_${ARCH}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/checksums.txt"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT

# Download binary
echo "Downloading ace_${OS}_${ARCH}..."
BINARY_PATH="${TMP_DIR}/ace"
if ! curl -fsSL "${BINARY_URL}" -o "${BINARY_PATH}"; then
    echo "Error: failed to download binary." >&2
    exit 1
fi

# Download checksums
echo "Downloading checksums..."
CHECKSUMS_PATH="${TMP_DIR}/checksums.txt"
if ! curl -fsSL "${CHECKSUMS_URL}" -o "${CHECKSUMS_PATH}"; then
    echo "Error: failed to download checksums." >&2
    exit 1
fi

# Verify checksum
echo "Verifying checksum..."
EXPECTED_HASH=$(grep "ace_${OS}_${ARCH}$" "${CHECKSUMS_PATH}" | awk '{print $1}')
if [ -z "$EXPECTED_HASH" ]; then
    echo "Error: checksum not found for ace_${OS}_${ARCH} in checksums.txt." >&2
    exit 1
fi

ACTUAL_HASH=$(sha256sum "${BINARY_PATH}" | awk '{print $1}')
if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
    echo "Error: checksum mismatch! Expected: ${EXPECTED_HASH}, Got: ${ACTUAL_HASH}. The binary may have been tampered with. Aborting." >&2
    exit 1
fi

echo "Checksum verified."

# Ensure install directory exists
if [ ! -d "${INSTALL_DIR}" ]; then
    mkdir -p "${INSTALL_DIR}"
fi

# Check if install dir is writable
if [ ! -w "${INSTALL_DIR}" ]; then
    echo "Error: cannot write to ${INSTALL_DIR}. Try with -b flag to specify a different directory." >&2
    exit 1
fi

# Install binary
INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}"
if [ -f "${INSTALL_PATH}" ]; then
    echo "Warning: replacing existing installation at ${INSTALL_PATH}"
fi

install -m 755 "${BINARY_PATH}" "${INSTALL_PATH}"

# Check if install dir is in PATH
add_to_path=""
case "${OS}" in
    linux)
        if [ -n "$HOME" ]; then
            if echo "$PATH" | grep -q "${HOME}/.local/bin"; then
                add_to_path=""
            else
                add_to_path="export PATH=\"\${HOME}/.local/bin:\${PATH}\" >> ~/.bashrc 2>/dev/null || true"
            fi
        fi
        ;;
    darwin)
        if [ -n "$HOME" ]; then
            if echo "$PATH" | grep -q "${HOME}/.local/bin"; then
                add_to_path=""
            else
                add_to_path="export PATH=\"\${HOME}/.local/bin:\${PATH}\" >> ~/.bashrc 2>/dev/null || true"
            fi
        fi
        ;;
esac

# Print success message
echo ""
echo "ACE ${LATEST_TAG} installed successfully!"
echo ""
echo "Installed to: ${INSTALL_PATH}"
echo "Data directory: ${HOME}/.local/share/ace/"
echo "Configuration: ${HOME}/.local/config/ace/"
echo ""
echo "Quick start:"
echo "  ace                    Start the server"
echo "  ace --help             Show all options"
echo "  ace paths              Show configured paths"
echo ""
echo "Documentation: https://ace.dev/docs"
echo ""

# PATH warning
if [ -n "$add_to_path" ]; then
    echo "NOTE: ${INSTALL_DIR} is not in your PATH."
    echo "Add it with: ${add_to_path}"
    echo "Or restart your shell for changes to take effect."
fi