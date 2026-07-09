#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD="$ROOT/build"
APP="$BUILD/bin/AndroidFileTransfer.app"
PY="/Library/Developer/CommandLineTools/usr/bin/python3"
BG_PY="/usr/local/bin/python3"
DMGBUILD="$PY -m dmgbuild"
VERSION="${1:-$(/usr/libexec/PlistBuddy -c 'Print CFBundleShortVersionString' "$APP/Contents/Info.plist" 2>/dev/null || echo "1.0.0")}"
DMG="$BUILD/bin/AndroidFileTransfer-${VERSION}.dmg"
ICNS="$BUILD/appicon.icns"

if [[ ! -d "$APP" ]]; then
  echo "Missing $APP — run 'wails build' first." >&2
  exit 1
fi

if ! $PY -c "import dmgbuild" 2>/dev/null; then
  echo "Installing dmgbuild..." >&2
  $PY -m pip install --user dmgbuild
fi

$BG_PY "$BUILD/generate-dmg-background.py"

if [[ ! -f "$ICNS" ]]; then
  ICONSET="$BUILD/appicon.iconset"
  rm -rf "$ICONSET"
  mkdir -p "$ICONSET"
  for size in 16 32 128 256 512; do
    sips -z "$size" "$size" "$BUILD/appicon.png" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
    double=$((size * 2))
    sips -z "$double" "$double" "$BUILD/appicon.png" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
  done
  iconutil -c icns "$ICONSET" -o "$ICNS"
  rm -rf "$ICONSET"
fi

rm -f "$DMG"
$DMGBUILD \
  -s "$BUILD/dmg-settings.py" \
  -Dbuild_dir="$BUILD" \
  "AndroidFileTransfer Installer" \
  "$DMG"

echo "Created $DMG"
