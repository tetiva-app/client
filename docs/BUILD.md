# Build & Distribution

## How builds are organized

Each platform has one build script that builds wherever it runs — in CI, in Docker on a Mac,
on a developer machine — and CI calls exactly that script:

| Platform | Build | Smoke check | Workflow |
|---|---|---|---|
| Windows | `build/release-windows.sh` | `build/windows/smoke.ps1` | `.github/workflows/windows.yml` |
| Linux | `build/release-linux.sh` | `build/linux/smoke.sh` | `.github/workflows/linux.yml` |
| macOS | `build/release-macos.sh` | — | — |

Release scripts write files under their release names, `Tetiva-X.Y.Z-…`. Every workflow runs
`build/check-versions.sh`, the release script, attests and uploads the result, then installs
and opens it on the target system. For a release, `bash build/fetch-ci-artifacts.sh <platform>`
takes the files of the green run of `main` into `bin/ci/<commit>/` and verifies each one.

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

### One running copy per data directory

At startup the app locks `<data dir>/tetiva.lock`; the data dir is `TETIVA_DATA_DIR` or
`~/.tetiva`. A second launch with the same data dir opens neither the database nor a window:
it passes its arguments (a `tetiva://` link, say) to the running copy over the loopback port
recorded in `<data dir>/instance.json`, prints `Tetiva is already running with <dir>…` and
exits 0. If the running copy does not answer within 5 seconds, it exits 2. A launch while the
running copy is shutting down waits, within the same 5 seconds, for it to exit and then starts
normally.

Dev and prod share `~/.tetiva`, so `wails3 dev` started while the installed app is open hands
over to it and quits. Give the dev build its own directory to run both at once:

```bash
TETIVA_DATA_DIR=/tmp/tetiva-dev wails3 dev
```

The same applies to `bin/client-server` and `bin/client-mcp`.

### Deep links

`tetiva://import?slug=<slug>[&token=<import-token>]` opens the import confirmation; the dev
bundle `bin/client.dev.app` answers to `tetiva-dev://` instead. The schemes are registered by
committed files, which `update:build-assets` would overwrite, so do not run it:
`build/config.yml` (`protocols`), `build/darwin/Info.plist` and `Info.dev.plist`
(`CFBundleURLTypes`), `build/windows/nsis/wails_tools.nsh` (protocol macros, quoted exe path)
and `build/linux/tetiva.desktop` (`%u`, `MimeType`).

`wails3 dev` starts the binary directly, so macOS learns about `tetiva-dev` only after an
explicit registration:

```bash
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f bin/client.dev.app
open 'tetiva-dev://import?slug=petstore-api-k3f9x2qa'
```

An app started by `open` gets the launchd environment, not your shell's, so it uses
`~/.tetiva` even if you exported `TETIVA_DATA_DIR`.

Two platform limits, both on the owner's release checklist:

- **macOS, the lock held by another copy.** The link arrives as an Apple Event, not in the
  arguments. If `~/.tetiva` is locked by a copy LaunchServices did not route the link to
  (`wails3 dev` while a link on share.tetiva.app starts `/Applications/Tetiva.app`,
  `bin/client.dev.app` beside the prod bundle, two copies of the bundle), the new process has no
  link to forward: the running copy comes to the front and no import confirmation opens. Wails
  keeps its Apple Event capture (`captureLaunchURL`) private, so there is nothing to reuse. Start
  `wails3 dev` with its own `TETIVA_DATA_DIR` so the copy LaunchServices opens owns `~/.tetiva`.
- **Linux under Wayland.** A link opened while Tetiva runs is handed over without an activation
  token, so GNOME and other compositors that block focus stealing may show a "Tetiva is ready"
  notification instead of raising the window. The import confirmation is already open when the
  user switches to it.

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
cross-compiles on Ubuntu with `build/release-windows.sh`, then installs each installer over
1.1.1 and opens the app on Windows x64 and ARM64. Take them with
`bash build/fetch-ci-artifacts.sh windows`.

Without CI: `bash build/release-windows.sh` (needs `makensis`: `brew install nsis`). It writes
`bin/Tetiva-X.Y.Z-windows-{amd64,arm64}-installer.exe`.

## Linux

`.github/workflows/linux.yml` builds `Tetiva-X.Y.Z-linux-{amd64,arm64}.deb` on every push to
`main` in an `ubuntu:22.04` container (glibc 2.35), then installs each package and opens the app
on Ubuntu 22.04 and 26.04; the amd64 package goes over 1.1.1. Take them with
`bash build/fetch-ci-artifacts.sh linux`.

Without CI, in Docker (arm64 builds natively on Apple Silicon in about a minute; amd64 takes
about ten under emulation and several GB of Docker disk):

```bash
docker build --platform linux/amd64 -t tetiva-linux-builder:22.04 -f build/linux/Dockerfile.builder build/linux
docker run --rm --platform linux/amd64 -v "$PWD":/src/client -v "$PWD/bin/linux":/src/client/bin \
  -v /src/client/frontend/node_modules -v /src/client/.task \
  -v tetiva-linux-gomod:/root/go/pkg/mod -v tetiva-linux-gocache:/root/.cache/go-build \
  tetiva-linux-builder:22.04 bash build/release-linux.sh
```

The container gets its own `bin/` (`bin/linux/` on the Mac), so the macOS build's `bin/client`
and the released files in `bin/` never meet the Linux build; the anonymous volumes keep the
Mac's `node_modules` and Task's checksums out as well. Smoke-check the result the way CI does:
`docker run --rm --platform linux/amd64 --shm-size=512m -v "$PWD":/src -w /src ubuntu:22.04 bash build/linux/smoke.sh bin/linux/Tetiva-X.Y.Z-linux-amd64.deb`.

Before a release, `bash build/check-versions.sh` confirms the version matches in every file that
carries it; CI runs the same check.
