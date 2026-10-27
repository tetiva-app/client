#!/usr/bin/env bash
set -euo pipefail
# Builds the .deb for this machine's arch where it runs (CI: ubuntu:22.04).
# Needs Go, Node, wails3, libgtk-3-dev, libwebkit2gtk-4.1-dev. Output: bin/Tetiva-*.deb
# PREVIEW=1 builds the tetiva-preview package instead: bin/Tetiva-Preview-*.deb

cd "$(dirname "$0")/.."
VERSION=$(grep -o 'AppVersion *= *"[^"]*"' internal/constants/app.go | cut -d'"' -f2)
PKG_VERSION=$(grep -o '^version: "[^"]*"' build/linux/nfpm/nfpm.yaml | cut -d'"' -f2)
[ "$VERSION" = "$PKG_VERSION" ] || { echo "version mismatch: constants $VERSION, nfpm $PKG_VERSION"; exit 1; }
ARCH=$(go env GOARCH)
PKG=tetiva
FILE=Tetiva
CONFIG=./build/linux/nfpm/nfpm.yaml
LDFLAGS="-w -s"
if [ "${PREVIEW:-}" = 1 ]; then
  PKG=tetiva-preview
  FILE=Tetiva-Preview
  CONFIG=./build/linux/nfpm/nfpm-preview.yaml
  LDFLAGS="$LDFLAGS -X github.com/tetiva-app/client/internal/constants.Flavor=preview"
  export TETIVA_VERSION="$VERSION"
fi
DEB="bin/${FILE}-${VERSION}-linux-${ARCH}.deb"

export PATH="$HOME/go/bin:$PATH"

wails3 task common:build:frontend BUILD_FLAGS='-tags production,gtk3'
CGO_ENABLED=1 go build -tags production,gtk3 -trimpath -buildvcs=false -ldflags="$LDFLAGS" -o bin/client .
GOARCH="$ARCH" wails3 tool package -name "$PKG" -format deb -config "$CONFIG" -out bin
mv "bin/$PKG.deb" "$DEB"
sha256sum "$DEB"
echo "DONE: $DEB"
