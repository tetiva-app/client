#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
[[ "${1:-}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "usage: $0 X.Y.Z"; exit 1; }
export V="$1"

perl -pi -e 's/(AppVersion\s*=\s*")[^"]+/${1}$ENV{V}/' internal/constants/app.go
perl -pi -e 's/^(\s*"version":\s*")[^"]+/${1}$ENV{V}/' frontend/package.json
# The first two are the package's own; the rest belong to dependencies.
perl -0777 -pi -e 's/("version":\s*")[^"]+/$n++ < 2 ? "$1$ENV{V}" : $&/ge' frontend/package-lock.json
perl -pi -e 's/^(\s+version:\s*")[^"]+/${1}$ENV{V}/' build/config.yml
perl -0777 -pi -e 's{(<key>CFBundle(?:ShortVersionString|Version)</key>\s*<string>)[^<]+}{$1$ENV{V}}g' build/darwin/Info.plist
perl -pi -e 's/^(version:\s*")[^"]+/${1}$ENV{V}/' build/linux/nfpm/nfpm.yaml
perl -pi -e 's/(define INFO_PRODUCTVERSION ")[^"]+/${1}$ENV{V}/' build/windows/nsis/wails_tools.nsh
perl -pi -e 's/("(?:file_version|ProductVersion)":\s*")[^"]+/${1}$ENV{V}/' build/windows/info.json
perl -pi -e 's/(<assemblyIdentity type="win32" name="yudinsv.com.Tetiva" version=")[^"]+/${1}$ENV{V}/' build/windows/wails.exe.manifest

if ! grep -qF "## [v$V]" CHANGELOG.md; then
  D=$(date +%F) perl -pi -e 's/^## \[Unreleased\]\n/$&\n## [v$ENV{V}] — $ENV{D}\n/' CHANGELOG.md
fi

bash build/check-versions.sh
