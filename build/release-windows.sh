#!/usr/bin/env bash
set -euo pipefail
# Cross-builds the Windows NSIS installers (amd64 + arm64) from macOS.
# ARCH vars don't propagate through `wails3 task`, hence the direct steps.
# Prereqs: makensis (brew install nsis). Output: bin/client-<arch>-installer.exe

cd "$(dirname "$0")/.."
export PATH="$HOME/go/bin:$PATH"

# info.json and wails_tools.nsh are generated assets and go stale on version
# bumps — sync them from config.yml instead of trusting the committed values.
VERSION=$(perl -ne 'print $1 if /^\s*version:\s*"([^"]+)"/' build/config.yml)
[ -n "$VERSION" ] || { echo "failed to read version from build/config.yml"; exit 1; }
perl -pi -e 's/("(?:file_version|ProductVersion)":\s*")[^"]+/${1}'"$VERSION"'/' build/windows/info.json
perl -pi -e 's/(<assemblyIdentity type="win32" name="[^"]+" version=")[^"]+/${1}'"$VERSION"'/' build/windows/wails.exe.manifest

wails3 task common:build:frontend

for ARCH in amd64 arm64; do
  wails3 generate syso -arch "$ARCH" -icon build/windows/icon.ico \
    -manifest build/windows/wails.exe.manifest -info build/windows/info.json \
    -out "wails_windows_${ARCH}.syso"
  GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 go build -tags production \
    -trimpath -buildvcs=false -ldflags="-w -s -H windowsgui" -o bin/client.exe
  rm -f "wails_windows_${ARCH}.syso"
  FLAG=$([ "$ARCH" = amd64 ] && echo AMD64 || echo ARM64)
  (cd build/windows/nsis && makensis -DINFO_PRODUCTVERSION="$VERSION" \
    -DARG_WAILS_${FLAG}_BINARY="$(pwd)/../../../bin/client.exe" project.nsi)
done
echo "DONE: bin/client-amd64-installer.exe bin/client-arm64-installer.exe"
