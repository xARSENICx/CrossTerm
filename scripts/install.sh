#!/bin/bash

# CrossTerm - Professional macOS Installer
# This script handles architecture detection and bypasses Gatekeeper flags.

set -e

# --- Configuration ---
BINARY_NAME="crossterm"
GITHUB_REPO="xARSENICx/CrossTerm"
INSTALL_DIR="/usr/local/bin"
# Update these URLs when you create your GitHub Release
BASE_URL="https://github.com/$GITHUB_REPO/releases/latest/download"

echo "🚀 Installing CrossTerm..."

# 1. Detect Architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    # Note: Ensure these filenames match what you upload to GitHub Releases
    URL="$BASE_URL/crossterm-apple-silicon"
    echo "  -> Detected Apple Silicon (M1/M2/M3)"
else
    URL="$BASE_URL/crossterm-intel"
    echo "  -> Detected Intel Architecture"
fi

# 2. Download Binary
echo "  -> Downloading binary..."
# Use -L to follow redirects (GitHub uses them for downloads)
# If the file doesn't exist yet, this may download a 404 page, 
# so we'll add a check in a production version.
curl -L -f -o "$BINARY_NAME" "$URL" || { echo "❌ Error: Could not download binary. Ensure your GitHub Release is public."; exit 1; }
chmod +x "$BINARY_NAME"

# 3. Clear macOS Quarantine (The "Magic" part)
if [ "$(uname)" = "Darwin" ]; then
    echo "  -> Optimizing for macOS security..."
    xattr -d com.apple.quarantine "$BINARY_NAME" 2>/dev/null || true
fi

# 4. Move to Path
echo "  -> Finalizing installation (may require password)..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
    sudo mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

echo ""
echo "✨ Installation complete! You can now run 'crossterm' from anywhere."
echo "   Try it now: crossterm"
