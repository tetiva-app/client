#!/usr/bin/env bash
set -euo pipefail
# Builds, signs, notarizes and staples the Tetiva macOS DMG.
# Prereqs: Developer ID cert in the keychain, notarytool profile "tetiva".

cd "$(dirname "$0")/.."
VERSION=$(grep -o 'AppVersion = "[^"]*"' internal/constants/app.go | cut -d'"' -f2)
IDENTITY="Developer ID Application: Saveliy Ludin (KB5J57CKFL)"
APP=bin/client.app
STAGE=bin/dmg-stage
DMG="bin/Tetiva-${VERSION}-macos-universal.dmg"

export PATH="$HOME/go/bin:$PATH"
wails3 task darwin:package:universal

rm -rf "$STAGE" && mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/Tetiva.app"

codesign --force --deep --options runtime --timestamp --sign "$IDENTITY" "$STAGE/Tetiva.app"
codesign --verify --deep --strict "$STAGE/Tetiva.app"

# Drag-to-install layout
ln -s /Applications "$STAGE/Applications"

rm -f "$DMG"
hdiutil create -volname "Tetiva" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
codesign --force --sign "$IDENTITY" "$DMG"

xcrun notarytool submit "$DMG" --keychain-profile tetiva --wait
xcrun stapler staple "$DMG"
spctl -a -t open --context context:primary-signature -v "$DMG" || true
echo "DONE: $DMG"
