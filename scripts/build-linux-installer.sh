#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"
OUTPUT_DIR="${1:-$ROOT_DIR/dist}"
STAGE_DIR="$OUTPUT_DIR/linux-amd64"
INSTALLER="$OUTPUT_DIR/Lunefetch-Setup-$VERSION-linux-amd64.run"

mkdir -p "$STAGE_DIR" "$OUTPUT_DIR"
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$STAGE_DIR/lunefetch" "$ROOT_DIR/"
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$STAGE_DIR/lunefetch-native-host" "$ROOT_DIR/cmd/native-host/"
cp "$ROOT_DIR/extension/icons/icon-128.png" "$STAGE_DIR/lunefetch.png"
cp "$ROOT_DIR/LICENSE" "$STAGE_DIR/LICENSE"

sed "s/@VERSION@/$VERSION/g" "$ROOT_DIR/scripts/linux-installer.sh" > "$OUTPUT_DIR/.linux-installer"
tar -czf "$OUTPUT_DIR/.lunefetch-payload.tar.gz" -C "$STAGE_DIR" lunefetch lunefetch-native-host lunefetch.png LICENSE
cat "$OUTPUT_DIR/.linux-installer" "$OUTPUT_DIR/.lunefetch-payload.tar.gz" > "$INSTALLER"
chmod 755 "$INSTALLER"
rm -rf "$STAGE_DIR" "$OUTPUT_DIR/.linux-installer" "$OUTPUT_DIR/.lunefetch-payload.tar.gz"
echo "$INSTALLER"
