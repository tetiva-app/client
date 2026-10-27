#!/usr/bin/env bash
set -euo pipefail
# Builds, signs, notarizes and staples the Tetiva macOS DMG and the in-app update zip.
# PREVIEW=1 builds Tetiva Preview instead: its own bundle id and DMG, no update zip.
# Prereqs: Developer ID cert in the keychain, notarytool profile "tetiva".

cd "$(dirname "$0")/.."
VERSION=$(grep -o 'AppVersion = "[^"]*"' internal/constants/app.go | cut -d'"' -f2)
IDENTITY="Developer ID Application: Saveliy Ludin (KB5J57CKFL)"
APP=bin/client.app
STAGE=bin/dmg-stage
NAME=Tetiva
BUNDLE_ID=yudinsv.com.Tetiva
if [ "${PREVIEW:-}" = 1 ]; then
  NAME="Tetiva Preview"
  BUNDLE_ID=yudinsv.com.Tetiva.preview
fi
DMG="bin/${NAME// /-}-${VERSION}-macos-universal.dmg"

export PATH="$HOME/go/bin:$PATH"
wails3 task darwin:package:universal
# Task picks up an exported EXTRA_TAGS=updatetest left over from the local update cycle.
if grep -qaF "$(cat build/updatesign/testdata/test-public.b64)" "$APP/Contents/MacOS/client"; then
  echo "$APP carries the updatetest public key" >&2
  exit 1
fi
# Task reads PREVIEW from the environment; this catches a bundle of the other flavor.
if [ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP/Contents/Info.plist")" != "$BUNDLE_ID" ]; then
  echo "$APP is not $BUNDLE_ID" >&2
  exit 1
fi

rm -rf "$STAGE" && mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/$NAME.app"

codesign --force --deep --options runtime --timestamp --sign "$IDENTITY" "$STAGE/$NAME.app"
codesign --verify --deep --strict "$STAGE/$NAME.app"

# Drag-to-install layout
ln -s /Applications "$STAGE/Applications"

rm -f "$DMG"
hdiutil create -volname "$NAME" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
codesign --force --sign "$IDENTITY" "$DMG"

xcrun notarytool submit "$DMG" --keychain-profile tetiva --wait
xcrun stapler staple "$DMG"
spctl -a -t open --context context:primary-signature -v "$DMG" || true

if [ "${PREVIEW:-}" = 1 ]; then
  echo "DONE: $DMG"
  exit 0
fi

xcrun stapler staple "$STAGE/Tetiva.app"
ZIP="bin/Tetiva-${VERSION}-macos-universal.zip"
rm -f "$ZIP"
ditto -c -k --keepParent "$STAGE/Tetiva.app" "$ZIP"
T=$(mktemp -d)
ditto -x -k "$ZIP" "$T"
codesign --verify --deep --strict -R="$(go run ./build/updatesign requirement)" "$T/Tetiva.app"
rm -rf "$T"
echo "DONE: $DMG $ZIP"
