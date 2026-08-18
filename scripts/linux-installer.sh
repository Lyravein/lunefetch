#!/bin/sh

set -eu

APP_NAME="Lunefetch"
VERSION="@VERSION@"
INSTALL_ROOT="${HOME}/.local/opt/lunefetch"
BIN_DIR="${HOME}/.local/bin"
APP_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/128x128/apps"
FIREFOX_DIR="${HOME}/.mozilla/native-messaging-hosts"
ZEN_DIR="${HOME}/.zen/native-messaging-hosts"
CHROMIUM_DIR="${HOME}/.config/chromium/NativeMessagingHosts"
CHROME_DIR="${HOME}/.config/google-chrome/NativeMessagingHosts"
BRAVE_DIR="${HOME}/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts"
EDGE_DIR="${HOME}/.config/microsoft-edge/NativeMessagingHosts"
VIVALDI_DIR="${HOME}/.config/vivaldi/NativeMessagingHosts"
HOST_NAME="com.lyravein.lunefetch"
PAYLOAD_LINE=$(awk '/^__LUNEFETCH_PAYLOAD__$/ { print NR + 1; exit }' "$0")
OPEN_STORE=true

manifest() {
  dir="$1"
  kind="$2"
  mkdir -p "$dir"
  if [ "$kind" = firefox ]; then
    cat > "$dir/$HOST_NAME.json" <<EOF
{
  "name": "$HOST_NAME",
  "description": "Lunefetch native messaging host",
  "path": "$BIN_DIR/lunefetch-native-host",
  "type": "stdio",
  "allowed_extensions": ["lunefetch@lyravein"]
}
EOF
  else
    cat > "$dir/$HOST_NAME.json" <<EOF
{
  "name": "$HOST_NAME",
  "description": "Lunefetch native messaging host",
  "path": "$BIN_DIR/lunefetch-native-host",
  "type": "stdio",
  "allowed_origins": ["chrome-extension://iidkhocioaefjlhhigiaphejnlidchke/"]
}
EOF
  fi
}

uninstall() {
  rm -f "$BIN_DIR/lunefetch" "$BIN_DIR/lunefetch-native-host"
  rm -f "$APP_DIR/lunefetch.desktop" "$ICON_DIR/lunefetch.png"
  for dir in "$FIREFOX_DIR" "$ZEN_DIR" "$CHROMIUM_DIR" "$CHROME_DIR" "$BRAVE_DIR" "$EDGE_DIR" "$VIVALDI_DIR"; do
    rm -f "$dir/$HOST_NAME.json"
  done
  rm -rf "$INSTALL_ROOT"
  printf '%s\n' "Lunefetch has been uninstalled. Browser extensions were left intact."
}

case "${1:-}" in
  --uninstall)
    uninstall
    exit 0
    ;;
  --no-open)
    OPEN_STORE=false
    ;;
  "") ;;
  *)
    printf '%s\n' "Usage: $0 [--no-open|--uninstall]" >&2
    exit 2
    ;;
esac

if [ "${PAYLOAD_LINE:-}" = "" ]; then
  printf '%s\n' "Installer payload is missing." >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
tail -n +"$PAYLOAD_LINE" "$0" | tar -xzf - -C "$tmp"

mkdir -p "$INSTALL_ROOT" "$BIN_DIR" "$APP_DIR" "$ICON_DIR"
cp "$tmp/lunefetch" "$INSTALL_ROOT/lunefetch"
cp "$tmp/lunefetch-native-host" "$INSTALL_ROOT/lunefetch-native-host"
cp "$tmp/lunefetch.png" "$ICON_DIR/lunefetch.png"
cp "$tmp/LICENSE" "$INSTALL_ROOT/LICENSE"
chmod 755 "$INSTALL_ROOT/lunefetch" "$INSTALL_ROOT/lunefetch-native-host"
ln -sfn "$INSTALL_ROOT/lunefetch" "$BIN_DIR/lunefetch"
ln -sfn "$INSTALL_ROOT/lunefetch-native-host" "$BIN_DIR/lunefetch-native-host"
cat > "$INSTALL_ROOT/uninstall" <<'EOF'
#!/bin/sh
set -eu
BIN_DIR="${HOME}/.local/bin"
APP_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/128x128/apps"
INSTALL_ROOT="${HOME}/.local/opt/lunefetch"
HOST_NAME="com.lyravein.lunefetch"
for dir in \
  "${HOME}/.mozilla/native-messaging-hosts" \
  "${HOME}/.zen/native-messaging-hosts" \
  "${HOME}/.config/chromium/NativeMessagingHosts" \
  "${HOME}/.config/google-chrome/NativeMessagingHosts" \
  "${HOME}/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts" \
  "${HOME}/.config/microsoft-edge/NativeMessagingHosts" \
  "${HOME}/.config/vivaldi/NativeMessagingHosts"; do
  rm -f "$dir/$HOST_NAME.json"
done
rm -f "$BIN_DIR/lunefetch" "$BIN_DIR/lunefetch-native-host"
rm -f "$APP_DIR/lunefetch.desktop" "$ICON_DIR/lunefetch.png"
rm -rf "$INSTALL_ROOT"
printf '%s\n' "Lunefetch has been uninstalled. Browser extensions were left intact."
EOF
chmod 755 "$INSTALL_ROOT/uninstall"

cat > "$APP_DIR/lunefetch.desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Lunefetch
GenericName=Download Manager
Comment=Fast, multi-threaded download manager
Exec=$BIN_DIR/lunefetch
Icon=lunefetch
Terminal=false
Categories=Network;FileTransfer;
Keywords=download;downloader;transfer;
StartupWMClass=lunefetch
StartupNotify=true
EOF

if command -v firefox >/dev/null 2>&1 || [ -d "$FIREFOX_DIR" ]; then manifest "$FIREFOX_DIR" firefox; fi
if command -v zen-browser >/dev/null 2>&1 || [ -d "$ZEN_DIR" ]; then manifest "$ZEN_DIR" firefox; fi
if command -v chromium >/dev/null 2>&1 || [ -d "$CHROMIUM_DIR" ]; then manifest "$CHROMIUM_DIR" chromium; fi
if command -v google-chrome >/dev/null 2>&1 || [ -d "$CHROME_DIR" ]; then manifest "$CHROME_DIR" chromium; fi
if command -v brave-browser >/dev/null 2>&1 || [ -d "$BRAVE_DIR" ]; then manifest "$BRAVE_DIR" chromium; fi
if command -v microsoft-edge >/dev/null 2>&1 || [ -d "$EDGE_DIR" ]; then manifest "$EDGE_DIR" chromium; fi
if command -v vivaldi >/dev/null 2>&1 || [ -d "$VIVALDI_DIR" ]; then manifest "$VIVALDI_DIR" chromium; fi

printf '\n%s %s installed.\n' "$APP_NAME" "$VERSION"
printf '%s\n' "Launch it from your application menu or run: $BIN_DIR/lunefetch"
printf '%s\n' "Install the browser extension separately from its official store:"
printf '%s\n' "  Firefox: https://addons.mozilla.org/en-US/firefox/addon/lunefetch/"
printf '%s\n' "The installer does not install browser extensions. Restart your browser after installing one."
if $OPEN_STORE && command -v xdg-open >/dev/null 2>&1 && { command -v firefox >/dev/null 2>&1 || [ -d "$FIREFOX_DIR" ]; }; then
  xdg-open 'https://addons.mozilla.org/en-US/firefox/addon/lunefetch/' >/dev/null 2>&1 &
fi
exit 0

__LUNEFETCH_PAYLOAD__
