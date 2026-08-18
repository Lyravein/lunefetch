#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"
DIST_DIR="$(mktemp -d)"
TEST_HOME="$(mktemp -d)"
trap 'rm -rf "$DIST_DIR" "$TEST_HOME"' EXIT

mkdir -p \
  "$TEST_HOME/.mozilla/native-messaging-hosts" \
  "$TEST_HOME/.zen/native-messaging-hosts" \
  "$TEST_HOME/.config/chromium/NativeMessagingHosts" \
  "$TEST_HOME/.config/google-chrome/NativeMessagingHosts" \
  "$TEST_HOME/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts" \
  "$TEST_HOME/.config/microsoft-edge/NativeMessagingHosts" \
  "$TEST_HOME/.config/vivaldi/NativeMessagingHosts"

"$ROOT_DIR/scripts/build-linux-installer.sh" "$DIST_DIR"
INSTALLER="$DIST_DIR/Lunefetch-Setup-$VERSION-linux-amd64.run"
test -x "$INSTALLER"

HOME="$TEST_HOME" "$INSTALLER" --no-open
APP="$TEST_HOME/.local/bin/lunefetch"
HOST="$TEST_HOME/.local/bin/lunefetch-native-host"
test -x "$APP"
test -x "$HOST"
"$HOST" --version | grep -Fx "lunefetch-native-host $VERSION"
test -x "$TEST_HOME/.local/opt/lunefetch/uninstall"
test -f "$TEST_HOME/.local/share/applications/lunefetch.desktop"
test -f "$TEST_HOME/.local/share/icons/hicolor/128x128/apps/lunefetch.png"

for dir in \
  "$TEST_HOME/.mozilla/native-messaging-hosts" \
  "$TEST_HOME/.zen/native-messaging-hosts" \
  "$TEST_HOME/.config/chromium/NativeMessagingHosts" \
  "$TEST_HOME/.config/google-chrome/NativeMessagingHosts" \
  "$TEST_HOME/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts" \
  "$TEST_HOME/.config/microsoft-edge/NativeMessagingHosts" \
  "$TEST_HOME/.config/vivaldi/NativeMessagingHosts"; do
  test -f "$dir/com.lyravein.lunefetch.json"
done

# Reinstall exercises the upgrade path and must preserve the stable identities.
HOME="$TEST_HOME" "$INSTALLER" --no-open
grep -Fq 'lunefetch@lyravein' "$TEST_HOME/.mozilla/native-messaging-hosts/com.lyravein.lunefetch.json"
grep -Fq 'iidkhocioaefjlhhigiaphejnlidchke' "$TEST_HOME/.config/chromium/NativeMessagingHosts/com.lyravein.lunefetch.json"

HOME="$TEST_HOME" "$TEST_HOME/.local/opt/lunefetch/uninstall"
test ! -e "$APP"
test ! -e "$HOST"
if find "$TEST_HOME" -name com.lyravein.lunefetch.json -print -quit | grep -q .; then
  echo "stale native messaging manifest remains after uninstall" >&2
  exit 1
fi
