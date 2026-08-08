#!/bin/bash
# build.sh — Build Lunefetch extension for Firefox and Chromium.
# Output: dist/lunefetch-firefox.zip, dist/lunefetch-chromium.zip

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$SCRIPT_DIR/dist"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-315532800}"

"$ROOT_DIR/scripts/check-version.sh"

mkdir -p "$DIST_DIR"

build() {
  local target="$1"          # firefox | chromium
  local manifest="$SCRIPT_DIR/manifests/${target}.json"
  local out="$DIST_DIR/lunefetch-${target}.zip"
  local tmp="$DIST_DIR/.build-${target}"
  local unpacked="$DIST_DIR/${target}-unpacked"

  echo "Building $target..."

  rm -rf "$tmp"
  mkdir -p "$tmp/icons"

  # Copy manifest as manifest.json
  cp "$manifest" "$tmp/manifest.json"

  # Copy shared background script
  cp "$SCRIPT_DIR/src/background.js" "$tmp/background.js"
  cp "$SCRIPT_DIR/src/core.mjs" "$tmp/core.mjs"
  cp "$SCRIPT_DIR/src/ui.mjs" "$tmp/ui.mjs"
  cp "$SCRIPT_DIR/src/ui.css" "$tmp/ui.css"
  cp "$SCRIPT_DIR/src/popup.html" "$tmp/popup.html"
  cp "$SCRIPT_DIR/src/popup.mjs" "$tmp/popup.mjs"
  cp "$SCRIPT_DIR/src/options.html" "$tmp/options.html"
  cp "$SCRIPT_DIR/src/options.mjs" "$tmp/options.mjs"
  cp "$SCRIPT_DIR/src/batch.html" "$tmp/batch.html"
  cp "$SCRIPT_DIR/src/batch.mjs" "$tmp/batch.mjs"

  # Icons are required by the action popup and notifications.
  cp "$SCRIPT_DIR/icons/"* "$tmp/icons/"

  rm -rf "$unpacked"
  cp -R "$tmp" "$unpacked"

  # Normalize mtimes and archive entries so identical sources produce identical bytes.
  while IFS= read -r -d '' file; do
    touch -d "@$SOURCE_DATE_EPOCH" "$file"
  done < <(find "$tmp" -type f -print0)

  # Package into zip with a stable, explicit file order and no host metadata.
  rm -f "$out"
  (
    cd "$tmp"
    printf '%s\n' \
      background.js core.mjs ui.mjs ui.css \
      popup.html popup.mjs options.html options.mjs batch.html batch.mjs \
      icons/icon-16.png icons/icon-32.png icons/icon-48.png icons/icon-128.png \
      manifest.json | zip -X -q "$out" -@
  )
  rm -rf "$tmp"

  echo "  -> $out"
}

build firefox
build chromium

(
  cd "$DIST_DIR"
  sha256sum lunefetch-firefox.zip lunefetch-chromium.zip > SHA256SUMS
)

sed "s/{{VERSION}}/$VERSION/g" "$SCRIPT_DIR/store/release-notes.md" > "$DIST_DIR/RELEASE_NOTES.md"

echo ""
echo "Done."
echo "  Firefox:  $DIST_DIR/lunefetch-firefox.zip"
echo "  Chromium: $DIST_DIR/lunefetch-chromium.zip"
echo "  Checksums: $DIST_DIR/SHA256SUMS"
