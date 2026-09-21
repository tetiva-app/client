#!/usr/bin/env bash
set -euo pipefail
# Compares every file a release bumps against AppVersion; CI runs it before building.

cd "$(dirname "$0")/.."
want=$(grep -o 'AppVersion *= *"[^"]*"' internal/constants/app.go | cut -d'"' -f2) || true
[ -n "$want" ] || { echo "internal/constants/app.go: AppVersion not found"; exit 1; }
fail=0

check() {
  if [ "$2" != "$want" ]; then
    echo "$1: ${2:-<none>}"
    fail=1
  fi
}

check frontend/package.json "$(node -p "require('./frontend/package.json').version")"
check "frontend/package-lock.json version" \
  "$(node -p "require('./frontend/package-lock.json').version")"
check "frontend/package-lock.json packages[\"\"].version" \
  "$(node -p "require('./frontend/package-lock.json').packages[''].version")"
check build/config.yml "$(perl -ne 'print $1 and exit if /^\s+version:\s*"([^"]+)"/' build/config.yml)"
for key in CFBundleShortVersionString CFBundleVersion; do
  check "build/darwin/Info.plist $key" \
    "$(perl -0777 -ne "print \$1 if m{<key>$key</key>\\s*<string>([^<]+)}" build/darwin/Info.plist)"
done
check build/linux/nfpm/nfpm.yaml "$(grep -o '^version: "[^"]*"' build/linux/nfpm/nfpm.yaml | cut -d'"' -f2)"
check build/windows/nsis/wails_tools.nsh \
  "$(perl -ne 'print $1 if /define INFO_PRODUCTVERSION "([^"]+)"/' build/windows/nsis/wails_tools.nsh)"
awk -v h="## [v$want]" 'index($0, h) == 1 { found = 1 } END { exit !found }' CHANGELOG.md ||
  { echo "CHANGELOG.md: no \"## [v$want]\" heading"; fail=1; }

if [ "$fail" -ne 0 ]; then
  echo "want $want from internal/constants/app.go"
fi
exit "$fail"
