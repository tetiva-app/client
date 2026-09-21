# Build & Distribution

## Prerequisites

- Go 1.26.1+
- Node.js + npm
- Wails v3: `~/go/bin/wails3`
- Developer ID certificate: `Developer ID Application: Saveliy Ludin (KB5J57CKFL)`
- Notary profile: `tetiva` (saved in Keychain)
- `fileicon` (brew): for DMG Applications folder icon

## Quick Build (dev)

```bash
cd client
export PATH="$HOME/go/bin:$PATH"
wails3 dev
```

## Production Build

### 1. Build + Sign + Notarize

```bash
cd client
export PATH="$HOME/go/bin:$PATH"
wails3 task darwin:sign:notarize
```

Output: `bin/client.app` — signed and notarized.

### 2. Build + Sign only (without notarization)

```bash
wails3 task darwin:sign
```

### 3. Build only (ad-hoc signature, local use)

```bash
wails3 task darwin:package
```

## Create DMG for Distribution

The full pipeline (universal build → sign → styled DMG → notarize → staple) is
automated:

```bash
build/release-macos.sh
```

Result: `bin/Tetiva-<version>-macos-universal.dmg` — ready to distribute.

Windows NSIS installers (amd64 + arm64, cross-built from macOS):

```bash
build/release-windows.sh
```

## Verification

### Check code signature

```bash
codesign -dvvv bin/client.app
```

Expected: `Authority=Developer ID Application: Saveliy Ludin (KB5J57CKFL)`

### Verify notarization (app)

```bash
spctl --assess --type execute -vvv bin/client.app
```

Expected: `accepted` + `source=Notarized Developer ID`

### Verify notarization (DMG)

```bash
spctl --assess --type open --context context:primary-signature -vvv bin/Tetiva-<version>-macos-universal.dmg
```

Expected: `accepted`

### Check stapled ticket

```bash
xcrun stapler validate bin/client.app
xcrun stapler validate bin/Tetiva-<version>-macos-universal.dmg
```

Expected: `The validate action worked!`

### Check Gatekeeper (simulate download)

```bash
# Add quarantine attribute (simulates downloading from internet)
xattr -w com.apple.quarantine "0082;$(printf '%x' $(date +%s));Safari;$(uuidgen)" bin/client.app

# Try to open — Gatekeeper should allow it
open bin/client.app
```

## Configuration

Signing config: `build/darwin/Taskfile.yml` (vars section)
App metadata: `build/config.yml`
Entitlements: `build/darwin/entitlements.plist` (sandbox OFF)
Bundle ID: `yudinsv.com.Tetiva`

## Gotchas

- **DMG must be notarized separately** from the `.app` — otherwise "cannot open" on other machines
- **Each rebuild invalidates** the notarization ticket — must re-notarize after every code change
- **Quarantine xattr** — downloaded apps get it; notarization + stapling ensures Gatekeeper clears it
- **Finder alias, not symlink** — symlinks show blank icon on macOS Tahoe; use `osascript` Finder alias
- **First-time notarization** for a new Developer ID can take 30+ min; subsequent: 1-3 min

## Windows

`.github/workflows/windows.yml` builds both installers on every push to `main`. It
cross-compiles on Ubuntu with `build/release-windows.sh`, then installs each installer
over 1.1.1 and opens the app on Windows x64 and ARM64.

Every push produces installers named after the current `AppVersion`, so take the ones
from the green run of the release commit and check they came from it:

```bash
SHA=$(git rev-parse main)
RUN=$(gh run list --workflow windows.yml --commit "$SHA" --status success --json databaseId --jq '.[0].databaseId')
gh run download "$RUN" -n windows-installers -D bin/
for f in bin/Tetiva-*-windows-*-installer.exe; do
  gh attestation verify "$f" --repo tetiva-app/client \
    --signer-workflow tetiva-app/client/.github/workflows/windows.yml \
    --source-ref refs/heads/main --source-digest "$SHA"
done
```

Without CI: `bash build/release-windows.sh` (needs `makensis`: `brew install nsis`). It
writes `bin/client-{amd64,arm64}-installer.exe`.

Before a release, `bash build/check-versions.sh` confirms the version matches in every
file that carries it; CI runs the same check.
