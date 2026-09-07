#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "VERSION must be semantic x.y.z, got: $VERSION" >&2
  exit 1
fi

for manifest in "$ROOT_DIR/extension/manifests/firefox.json" "$ROOT_DIR/extension/manifests/chromium.json"; do
  manifest_version="$(node -p "JSON.parse(require('fs').readFileSync(process.argv[1], 'utf8')).version" "$manifest")"
  if [[ "$manifest_version" != "$VERSION" ]]; then
    echo "Version drift: $manifest declares $manifest_version, expected $VERSION" >&2
    exit 1
  fi
done

if [[ -n "${GITHUB_REF_NAME:-}" && "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
  # Tags may carry a prerelease suffix (v0.1.0-beta for VERSION 0.1.0).
  if [[ ! "$GITHUB_REF_NAME" =~ ^v$VERSION(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "Release tag $GITHUB_REF_NAME does not match VERSION v$VERSION" >&2
    exit 1
  fi
fi

echo "Version $VERSION is consistent."
