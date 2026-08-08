#!/bin/bash
# install.sh — Install Lunefetch native messaging host for Firefox and/or Chromium-based browsers.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="$(tr -d '[:space:]' < "$SCRIPT_DIR/VERSION")"
BINARY_NAME="lunefetch-native-host"
BINARY_DST="$HOME/.local/bin/$BINARY_NAME"

FIREFOX_MANIFEST_CONTENT() {
  cat <<EOF
{
  "name": "com.lyravein.lunefetch",
  "description": "Lunefetch native messaging host",
  "path": "$BINARY_DST",
  "type": "stdio",
  "allowed_extensions": ["lunefetch@lyravein"]
}
EOF
}

CHROMIUM_MANIFEST_CONTENT() {
  cat <<EOF
{
  "name": "com.lyravein.lunefetch",
  "description": "Lunefetch native messaging host",
  "path": "$BINARY_DST",
  "type": "stdio",
  "allowed_origins": ["chrome-extension://iidkhocioaefjlhhigiaphejnlidchke/"]
}
EOF
}

# Native messaging host directories per browser.
FIREFOX_DIR="$HOME/.mozilla/native-messaging-hosts"
ZEN_DIR="$HOME/.zen/native-messaging-hosts"
CHROMIUM_DIR="$HOME/.config/chromium/NativeMessagingHosts"
CHROME_DIR="$HOME/.config/google-chrome/NativeMessagingHosts"
BRAVE_DIR="$HOME/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts"

MANIFEST_FILE="com.lyravein.lunefetch.json"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
INSTALL_FIREFOX=false
INSTALL_CHROMIUM=false
UNINSTALL=false
AUTO_DETECT=false

if [[ $# -eq 0 ]]; then
  # No args: install for all detected browsers.
  INSTALL_FIREFOX=true
  INSTALL_CHROMIUM=true
  AUTO_DETECT=true
else
  for arg in "$@"; do
    case "$arg" in
      --firefox)  INSTALL_FIREFOX=true ;;
      --chromium) INSTALL_CHROMIUM=true ;;
      --chrome)   INSTALL_CHROMIUM=true ;;  # alias
      --uninstall) UNINSTALL=true ;;
      *)
        echo "Usage: $0 [--firefox] [--chromium] [--uninstall]"
        echo "  No flags = install for all detected browsers."
        exit 1
        ;;
    esac
  done
fi

EDGE_DIR="$HOME/.config/microsoft-edge/NativeMessagingHosts"
VIVALDI_DIR="$HOME/.config/vivaldi/NativeMessagingHosts"

if $UNINSTALL; then
  echo "Removing Lunefetch native messaging integration..."
  for dir in "$FIREFOX_DIR" "$ZEN_DIR" "$CHROMIUM_DIR" "$CHROME_DIR" "$BRAVE_DIR" "$EDGE_DIR" "$VIVALDI_DIR"; do
    rm -f "$dir/$MANIFEST_FILE"
  done
  rm -f "$BINARY_DST"
  echo "Uninstall complete. Browser extension data was left intact."
  exit 0
fi

# ---------------------------------------------------------------------------
# Build native host binary
# ---------------------------------------------------------------------------
echo "Building native host binary..."
mkdir -p "$(dirname "$BINARY_DST")"
(cd "$SCRIPT_DIR" && go build -ldflags "-X main.version=$VERSION" -o "$BINARY_DST" ./cmd/native-host/)
chmod +x "$BINARY_DST"
echo "  -> $BINARY_DST"

# ---------------------------------------------------------------------------
# Install manifest helper
# ---------------------------------------------------------------------------
install_manifest() {
  local dir="$1"
  local label="$2"
  local browser="$3"
  local force="${4:-}"
  if [[ -d "$dir" ]] || [[ "$force" == "force" ]]; then
    mkdir -p "$dir"
    if [[ "$browser" == "firefox" ]]; then
      FIREFOX_MANIFEST_CONTENT > "$dir/$MANIFEST_FILE"
    else
      CHROMIUM_MANIFEST_CONTENT > "$dir/$MANIFEST_FILE"
    fi
    echo "  -> $label: $dir/$MANIFEST_FILE"
  else
    echo "  (skipping $label — directory not found)"
  fi
}

# ---------------------------------------------------------------------------
# Firefox-based browsers
# ---------------------------------------------------------------------------
if $INSTALL_FIREFOX; then
  echo ""
  echo "Installing for Firefox-based browsers..."
  if $AUTO_DETECT; then
    install_manifest "$FIREFOX_DIR" "Firefox" firefox
  else
    install_manifest "$FIREFOX_DIR" "Firefox" firefox force
  fi
  install_manifest "$ZEN_DIR"     "Zen" firefox
fi

# ---------------------------------------------------------------------------
# Chromium-based browsers
# ---------------------------------------------------------------------------
if $INSTALL_CHROMIUM; then
  echo ""
  echo "Installing for Chromium-based browsers..."
  install_manifest "$CHROMIUM_DIR" "Chromium" chromium
  install_manifest "$CHROME_DIR"   "Google Chrome" chromium
  install_manifest "$BRAVE_DIR"    "Brave" chromium
  install_manifest "$EDGE_DIR"     "Microsoft Edge" chromium
  install_manifest "$VIVALDI_DIR"  "Vivaldi" chromium
fi

# ---------------------------------------------------------------------------
# Build extension zips
# ---------------------------------------------------------------------------
echo ""
echo "Building extension packages..."
chmod +x "$SCRIPT_DIR/extension/build.sh"
"$SCRIPT_DIR/extension/build.sh"

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "Installation complete!"
echo ""
if $INSTALL_FIREFOX; then
  echo "Firefox / Zen:"
  echo "  1. Open about:debugging -> This Firefox"
  echo "  2. Load Temporary Add-on -> select extension/dist/lunefetch-firefox.zip"
fi
if $INSTALL_CHROMIUM; then
  echo "Chromium / Chrome / Brave:"
  echo "  1. Open chrome://extensions -> Enable Developer mode"
  echo "  2. Load unpacked -> or drag extension/dist/lunefetch-chromium.zip"
fi
echo ""
echo "  3. Start Lunefetch: ./lunefetch"
echo "  4. Try downloading a file — it will be intercepted automatically."
