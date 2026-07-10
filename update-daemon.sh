#!/usr/bin/env bash
#
# update-daemon.sh — Update the Multica daemon CLI to the latest version.
#
# Usage:
#   curl -fsSL https://multica.binguosoft.net/update-daemon.sh | bash
#
set -euo pipefail

DOWNLOAD_URL="https://multica.binguosoft.net/multica"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="multica"

log() { echo "[$(date '+%H:%M:%S')] $*"; }

# Detect system architecture
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
log "System: $OS $ARCH"

# Check if running as root or with sudo
if [ "$EUID" -ne 0 ] && ! command -v sudo &>/dev/null; then
  echo "Error: This script requires root privileges or sudo."
  echo "Please run with: sudo bash update-daemon.sh"
  exit 1
fi

# Detect current version
CURRENT_VERSION=""
if command -v "$BINARY_NAME" &>/dev/null; then
  CURRENT_VERSION=$($BINARY_NAME version 2>/dev/null | head -1 | awk '{print $2}') || true
  log "Current version: ${CURRENT_VERSION:-unknown}"
else
  log "Multica CLI not found in PATH, installing..."
fi

# Download new version to a temporary directory
log "Downloading latest version from $DOWNLOAD_URL..."
TEMP_DIR=$(mktemp -d)
TEMP_FILE="$TEMP_DIR/multica"
trap "rm -rf $TEMP_DIR" EXIT

if command -v curl &>/dev/null; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_FILE" || {
    echo "Error: Failed to download CLI binary."
    exit 1
  }
elif command -v wget &>/dev/null; then
  wget -q "$DOWNLOAD_URL" -O "$TEMP_FILE" || {
    echo "Error: Failed to download CLI binary."
    exit 1
  }
else
  echo "Error: curl or wget is required."
  exit 1
fi

# Verify file size (should be around 14MB)
FILE_SIZE=$(stat -c%s "$TEMP_FILE" 2>/dev/null || stat -f%z "$TEMP_FILE" 2>/dev/null || echo "0")
if [ "$FILE_SIZE" -lt 1000000 ]; then
  echo "Error: Downloaded file is too small (${FILE_SIZE} bytes). Download may have failed."
  exit 1
fi
log "Downloaded ${FILE_SIZE} bytes"

# Make executable
chmod +x "$TEMP_FILE"

# Check file type
FILE_TYPE=$(file "$TEMP_FILE" 2>/dev/null || echo "unknown")
log "File type: $FILE_TYPE"

# Verify the download - try to run version command
NEW_VERSION=$("$TEMP_FILE" version 2>&1 | head -1 | awk '{print $2}') || {
  # If execution fails, show detailed error
  echo ""
  echo "Error: Downloaded file is not a valid Multica binary for this system."
  echo ""
  echo "System info:"
  echo "  Architecture: $ARCH"
  echo "  OS: $OS"
  echo "  File: $FILE_TYPE"
  echo ""
  echo "This usually means the binary is not compatible with your system architecture."
  echo "Current binary is built for: linux x86-64"
  echo ""
  echo "Solutions:"
  echo "  1. If you're on ARM/aarch64, a different binary is needed"
  echo "  2. Check https://github.com/multica-ai/multica/releases for other architectures"
  echo "  3. Build from source: go build -o multica ./cmd/multica"
  exit 1
}

log "New version: $NEW_VERSION"

# Check if update is needed
if [ "$CURRENT_VERSION" = "$NEW_VERSION" ]; then
  log "Already up to date!"
  exit 0
fi

# Stop daemon if running
if command -v "$BINARY_NAME" &>/dev/null; then
  log "Stopping daemon before update..."
  $BINARY_NAME daemon stop 2>/dev/null || true
fi

# Install
log "Installing to $INSTALL_DIR/$BINARY_NAME..."
if [ "$EUID" -eq 0 ]; then
  cp "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
else
  sudo cp "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
fi

log "✅ Multica CLI updated successfully!"
log "   Version: $NEW_VERSION"
log ""
log "Next steps:"
log "  1. Run 'multica daemon start' to start the daemon"
log "  2. Run 'multica daemon status' to check daemon status"
log "  3. Run 'multica --help' for available commands"
