#!/usr/bin/env bash
set -euo pipefail
# Builds both NSIS installers where it runs; ARCH does not pass through `wails3 task`.
# Needs makensis (nsis). Output: bin/Tetiva-<ver>-windows-<arch>-installer<OUT_SUFFIX>.exe
# PREVIEW=1 builds Tetiva Preview, named Tetiva-Preview-* instead of Tetiva-*.

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

FILE=Tetiva
EXE=client
ICON=build/windows/icon.ico
INFO=build/windows/info.json
MANIFEST=build/windows/wails.exe.manifest
LDFLAGS="-w -s -H windowsgui"
NSIS_PREVIEW=
if [ "${PREVIEW:-}" = 1 ]; then
  FILE=Tetiva-Preview
  EXE=client-preview
  ICON=build/windows/icon-preview.ico
  INFO=bin/preview/info.json
  MANIFEST=bin/preview/wails.exe.manifest
  LDFLAGS="$LDFLAGS -X github.com/tetiva-app/client/internal/constants.Flavor=preview"
  NSIS_PREVIEW=-DTETIVA_PREVIEW
fi

if [ "$STAGE" != installers ]; then
  # info.json and wails_tools.nsh are generated assets and go stale on version
  # bumps — sync them from config.yml instead of trusting the committed values.
  perl -pi -e 's/("(?:file_version|FileVersion|ProductVersion)":\s*")[^"]+/${1}'"$VERSION"'/' build/windows/info.json
  perl -pi -e 's/(<assemblyIdentity type="win32" name="yudinsv.com.Tetiva" version=")[^"]+/${1}'"$VERSION"'/' build/windows/wails.exe.manifest
  if [ "${PREVIEW:-}" = 1 ]; then
    mkdir -p bin/preview
    perl -pe 's/("(?:ProductName|FileDescription)":\s*")[^"]+/${1}Tetiva Preview/' build/windows/info.json > "$INFO"
    perl -pe 's/name="yudinsv.com.Tetiva"/name="yudinsv.com.Tetiva.preview"/' build/windows/wails.exe.manifest > "$MANIFEST"
  fi

  # GOOS and tags of the .exe below: bindings match it, and Linux cgo files stay out.
  GOOS=windows wails3 task common:build:frontend BUILD_FLAGS="-tags $TAGS"

  for ARCH in amd64 arm64; do
    wails3 generate syso -arch "$ARCH" -icon "$ICON" \
      -manifest "$MANIFEST" -info "$INFO" \
      -out "wails_windows_${ARCH}.syso"
    GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 go build -tags "$TAGS" \
      -trimpath -buildvcs=false -ldflags="$LDFLAGS" -o "bin/${EXE}-${ARCH}${OUT_SUFFIX}.exe"
    rm -f "wails_windows_${ARCH}.syso"
  done
fi
[ "$STAGE" = exes ] && { echo "DONE: bin/${EXE}-{amd64,arm64}${OUT_SUFFIX}.exe"; exit 0; }

# Gitignored, and makensis embeds it into every installer.
wails3 generate webview2bootstrapper -dir build/windows/nsis

for ARCH in amd64 arm64; do
  FLAG=$([ "$ARCH" = amd64 ] && echo AMD64 || echo ARM64)
  (cd build/windows/nsis && makensis ${NSIS_PREVIEW:+"$NSIS_PREVIEW"} -DINFO_PRODUCTVERSION="$VERSION" \
    -DARG_WAILS_${FLAG}_BINARY="$(pwd)/../../../bin/${EXE}-${ARCH}${OUT_SUFFIX}.exe" project.nsi)
  mv "bin/client-${ARCH}-installer.exe" "bin/${FILE}-${VERSION}-windows-${ARCH}-installer${OUT_SUFFIX}.exe"
done
echo "DONE: bin/${FILE}-${VERSION}-windows-{amd64,arm64}-installer${OUT_SUFFIX}.exe"
