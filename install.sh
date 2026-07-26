#!/bin/bash
# install.sh - Install lunefetch native messaging host for Firefox/Zen

set -e

BINARY_SRC="./dm-native-host"
BINARY_DST="/usr/local/bin/dm-native-host"
MANIFEST_SRC="./native-host-manifest.json"
MANIFEST_DIR="$HOME/.mozilla/native-messaging-hosts"
MANIFEST_DST="$MANIFEST_DIR/com.lyravein.download_manager.json"

echo "Building native host binary..."
go build -o "$BINARY_SRC" ./cmd/native-host/

echo "Installing binary to $BINARY_DST (needs sudo)..."
sudo install -m 755 "$BINARY_SRC" "$BINARY_DST"
rm "$BINARY_SRC"

echo "Installing native messaging manifest to $MANIFEST_DST..."
mkdir -p "$MANIFEST_DIR"

# Update path in manifest to point to the installed binary.
sed "s|/usr/local/bin/dm-native-host|$BINARY_DST|g" "$MANIFEST_SRC" > "$MANIFEST_DST"

echo ""
echo "Done! Next steps:"
echo "  1. Open Firefox/Zen → about:debugging → This Firefox"
echo "  2. Click 'Load Temporary Add-on' → select extension/manifest.json"
echo "  3. Start lunefetch: ./lunefetch"
echo "  4. Try downloading a file in the browser — it will be captured."
