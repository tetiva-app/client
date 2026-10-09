#!/usr/bin/env bash
set -euo pipefail
# Downloads a platform's files from a commit's green CI run and verifies their provenance.
# Usage: fetch-ci-artifacts.sh <windows|linux> [<commit>]. Output: bin/ci/<commit>/

cd "$(dirname "$0")/.."
platform=$1
sha=$(git rev-parse "${2:-main}")
version=$(git show "$sha:internal/constants/app.go" | grep -o 'AppVersion *= *"[^"]*"' | cut -d'"' -f2)
artifact=""
case $platform in
  windows)
    names="Tetiva-$version-windows-amd64-installer.exe Tetiva-$version-windows-arm64-installer.exe"
    artifact=windows-installers-release-signing ;;
  linux) names="Tetiva-$version-linux-amd64.deb Tetiva-$version-linux-arm64.deb" ;;
  *) echo "platform must be windows or linux"; exit 1 ;;
esac
run=""
for id in $(gh run list --workflow "$platform.yml" --commit "$sha" --status success --json databaseId --jq '.[].databaseId'); do
  if [ -z "$artifact" ] || gh api "repos/tetiva-app/client/actions/runs/$id/artifacts" --jq '.artifacts[].name' | grep -qx "$artifact"; then
    run=$id
    break
  fi
done
[ -n "$run" ] || { echo "no green $platform run for $sha${artifact:+ with $artifact}"; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
if [ -n "$artifact" ]; then
  gh run download "$run" -n "$artifact" -D "$tmp/$artifact"
else
  gh run download "$run" -D "$tmp"
fi
for name in $names; do
  found=$(find "$tmp" -mindepth 2 -type f -name "$name")
  [ "$(printf '%s' "$found" | grep -c .)" -eq 1 ] || { echo "$name: not exactly once in run $run"; exit 1; }
  gh attestation verify "$found" --repo tetiva-app/client \
    --signer-workflow "tetiva-app/client/.github/workflows/$platform.yml" \
    --source-ref refs/heads/main --source-digest "$sha" >/dev/null
  mv "$found" "$tmp/$name"
done

out=bin/ci/$sha
mkdir -p "$out"
for name in $names; do
  mv -f "$tmp/$name" "$out/$name"
  echo "verified $out/$name"
done
