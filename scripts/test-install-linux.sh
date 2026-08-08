#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_HOME="$(mktemp -d)"
trap 'rm -rf "$TEST_HOME"' EXIT

mkdir -p \
  "$TEST_HOME/.mozilla/native-messaging-hosts" \
  "$TEST_HOME/.zen/native-messaging-hosts" \
  "$TEST_HOME/.config/chromium/NativeMessagingHosts" \
  "$TEST_HOME/.config/google-chrome/NativeMessagingHosts" \
  "$TEST_HOME/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts" \
  "$TEST_HOME/.config/microsoft-edge/NativeMessagingHosts" \
  "$TEST_HOME/.config/vivaldi/NativeMessagingHosts"

HOME="$TEST_HOME" "$ROOT_DIR/install.sh"
HOST="$TEST_HOME/.local/bin/lunefetch-native-host"
test -x "$HOST"
"$HOST" --version | grep -Fx "lunefetch-native-host $(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

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
HOME="$TEST_HOME" "$ROOT_DIR/install.sh"
grep -Fq 'lunefetch@lyravein' "$TEST_HOME/.mozilla/native-messaging-hosts/com.lyravein.lunefetch.json"
grep -Fq 'iidkhocioaefjlhhigiaphejnlidchke' "$TEST_HOME/.config/chromium/NativeMessagingHosts/com.lyravein.lunefetch.json"

HOME="$TEST_HOME" "$ROOT_DIR/install.sh" --uninstall
test ! -e "$HOST"
if find "$TEST_HOME" -name com.lyravein.lunefetch.json -print -quit | grep -q .; then
  echo "stale native messaging manifest remains after uninstall" >&2
  exit 1
fi
