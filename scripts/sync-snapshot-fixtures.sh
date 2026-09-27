#!/usr/bin/env bash
set -euo pipefail
# Tests read these copies, so a clean clone passes without the sibling proto repo.

cd "$(dirname "$0")/.."
src=../proto-tetiva/snapshot
dst=testdata/snapshot
files=(fixtures/all-protocols.json fixtures/max-size.json collection-snapshot.v1.schema.json)

[ -d "$src" ] || { echo "$src not found"; exit 1; }

if [ "${1:-}" = "--check" ]; then
  fail=0
  for f in "${files[@]}"; do
    name=$(basename "$f")
    if ! cmp -s "$src/$f" "$dst/$name"; then
      echo "$dst/$name differs from $src/$f"
      fail=1
    fi
  done
  exit "$fail"
fi

mkdir -p "$dst"
for f in "${files[@]}"; do
  cp "$src/$f" "$dst/$(basename "$f")"
done
