#!/usr/bin/env bash
set -euo pipefail
# Builds both NSIS installers where it runs; ARCH does not pass through `wails3 task`.
# Needs makensis (nsis). Output: bin/Tetiva-<ver>-windows-<arch>-installer<OUT_SUFFIX>.exe

cd "$(dirname "$0")/.."
export PATH="$HOME/go/bin:$PATH"
STAGE="${1:-all}"
TAGS="${TAGS:-production}"
OUT_SUFFIX="${OUT_SUFFIX:-}"
case $STAGE in
  all | exes | installers) ;;
  *) echo "stage must be exes or installers"; exit 1 ;;
esac

VERSION=$(perl -ne 'print $1 if /^\s*version:\s*"([^"]+)"/' build/config.yml)
[ -n "$VERSION" ] || { echo "failed to read version from build/config.yml"; exit 1; }

if [ "$STAGE" != installers ]; then
  # info.json and wails_tools.nsh are generated assets and go stale on version
  # bumps — sync them from config.yml instead of trusting the committed values.
  perl -pi -e 's/("(?:file_version|FileVersion|ProductVersion)":\s*")[^"]+/${1}'"$VERSION"'/' build/windows/info.json
  perl -pi -e 's/(<assemblyIdentity type="win32" name="yudinsv.com.Tetiva" version=")[^"]+/${1}'"$VERSION"'/' build/windows/wails.exe.manifest

  # GOOS and tags of the .exe below: bindings match it, and Linux cgo files stay out.
  GOOS=windows wails3 task common:build:frontend BUILD_FLAGS="-tags $TAGS"

  for ARCH in amd64 arm64; do
    wails3 generate syso -arch "$ARCH" -icon build/windows/icon.ico \
      -manifest build/windows/wails.exe.manifest -info build/windows/info.json \
      -out "wails_windows_${ARCH}.syso"
    GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 go build -tags "$TAGS" \
      -trimpath -buildvcs=false -ldflags="-w -s -H windowsgui" -o "bin/client-${ARCH}${OUT_SUFFIX}.exe"
    rm -f "wails_windows_${ARCH}.syso"
  done
fi
[ "$STAGE" = exes ] && { echo "DONE: bin/client-{amd64,arm64}${OUT_SUFFIX}.exe"; exit 0; }

# Gitignored, and makensis embeds it into every installer.
wails3 generate webview2bootstrapper -dir build/windows/nsis

for ARCH in amd64 arm64; do
  FLAG=$([ "$ARCH" = amd64 ] && echo AMD64 || echo ARM64)
  (cd build/windows/nsis && makensis -DINFO_PRODUCTVERSION="$VERSION" \
    -DARG_WAILS_${FLAG}_BINARY="$(pwd)/../../../bin/client-${ARCH}${OUT_SUFFIX}.exe" project.nsi)
  mv "bin/client-${ARCH}-installer.exe" "bin/Tetiva-${VERSION}-windows-${ARCH}-installer${OUT_SUFFIX}.exe"
done
echo "DONE: bin/Tetiva-${VERSION}-windows-{amd64,arm64}-installer${OUT_SUFFIX}.exe"
