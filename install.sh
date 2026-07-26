#!/bin/bash
# install.sh - Install Lunefetch native messaging host for Firefox/Zen

set -e

BINARY_DST="$HOME/.local/bin/dm-native-host"
MANIFEST_DIR="$HOME/.mozilla/native-messaging-hosts"
MANIFEST_DST="$MANIFEST_DIR/com.lyravein.lunefetch.json"

echo "Building native host binary..."
go build -o "$BINARY_DST" ./cmd/native-host/
chmod +x "$BINARY_DST"
echo "Binary installed to $BINARY_DST"

echo "Installing native messaging manifest to $MANIFEST_DST..."
mkdir -p "$MANIFEST_DIR"

cat > "$MANIFEST_DST" <<EOF
{
  "name": "com.lyravein.lunefetch",
  "description": "Lunefetch native messaging host",
  "path": "$BINARY_DST",
  "type": "stdio",
  "allowed_extensions": ["lunefetch@lyravein"]
}
EOF

echo ""
echo "Done! Next steps:"
echo "  1. Open Firefox/Zen -> about:debugging -> This Firefox"
echo "  2. Click 'Load Temporary Add-on' -> select extension/manifest.json"
echo "  3. Start Lunefetch: ./lunefetch"
echo "  4. Try downloading a file in the browser -- it will be captured."
