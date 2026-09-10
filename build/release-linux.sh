#!/usr/bin/env bash
set -euo pipefail
# Builds the Linux .deb natively on an Ubuntu 22.04 host so the binary stays on glibc <= 2.35.
# The frontend is built here (no node on the build host) and shipped inside frontend/dist.
# Prereqs: ssh to $TETIVA_LINUX_HOST, sibling ../proto-tetiva checkout (go.mod replace).

cd "$(dirname "$0")/.."
VERSION=$(grep -o 'AppVersion = "[^"]*"' internal/constants/app.go | cut -d'"' -f2)
PKG_VERSION=$(grep -o '^version: "[^"]*"' build/linux/nfpm/nfpm.yaml | cut -d'"' -f2)
[ "$VERSION" = "$PKG_VERSION" ] || { echo "version mismatch: constants $VERSION, nfpm $PKG_VERSION"; exit 1; }

REMOTE="${TETIVA_LINUX_HOST:-yudinsv@10.2.0.204}"
REMOTE_DIR="tetiva-build"
DEB="Tetiva-${VERSION}-linux-amd64.deb"
GO_TOOL="go1.26.1"
WAILS_VERSION="v3.0.0-beta.16"

export PATH="$HOME/go/bin:$PATH"
# A proxy in the shell profile breaks ssh/rsync to the LAN build host.
export HTTPS_PROXY="" https_proxy="" HTTP_PROXY="" http_proxy=""

wails3 task common:build:frontend BUILD_FLAGS='-tags production'

RSYNC_EXCLUDES=(--exclude .git --exclude node_modules --exclude bin --exclude frontend/bindings
                --exclude frontend/playwright-report --exclude frontend/test-results)
ssh "$REMOTE" "mkdir -p $REMOTE_DIR/client $REMOTE_DIR/proto-tetiva"
rsync -a --delete "${RSYNC_EXCLUDES[@]}" ./ "$REMOTE:$REMOTE_DIR/client/"
rsync -a --delete --exclude .git ../proto-tetiva/ "$REMOTE:$REMOTE_DIR/proto-tetiva/"

ssh "$REMOTE" "REMOTE_DIR=$REMOTE_DIR DEB=$DEB GO_TOOL=$GO_TOOL WAILS_VERSION=$WAILS_VERSION bash -s" <<'REMOTE'
set -euo pipefail
export PATH="$HOME/go/bin:$PATH"

command -v "$GO_TOOL" >/dev/null || { go install "golang.org/dl/$GO_TOOL@latest"; "$GO_TOOL" download; }
# CGO off: the CLI pulls in the GTK4 webkit probe, which won't compile on 22.04.
wails3 version 2>/dev/null | grep -q "${WAILS_VERSION#v}" || \
  CGO_ENABLED=0 "$GO_TOOL" install "github.com/wailsapp/wails/v3/cmd/wails3@$WAILS_VERSION"

cd "$HOME/$REMOTE_DIR/client"
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 "$GO_TOOL" build -tags production,gtk3 \
  -trimpath -buildvcs=false -ldflags="-w -s" -o bin/client .
GOARCH=amd64 wails3 tool package -name tetiva -format deb -config ./build/linux/nfpm/nfpm.yaml -out bin
mv "bin/tetiva.deb" "bin/$DEB"
REMOTE

mkdir -p bin
rsync -a "$REMOTE:$REMOTE_DIR/client/bin/$DEB" bin/
shasum -a 256 "bin/$DEB"
echo "DONE: $(pwd)/bin/$DEB"
