# Build & Distribution

## How builds are organized

Each platform has one build script that builds wherever it runs — in CI, in Docker on a Mac,
on a developer machine — and CI calls exactly that script:

| Platform | Build | Smoke check | Workflow |
|---|---|---|---|
| Windows | `build/release-windows.sh` | `build/windows/smoke.ps1`, `build/windows/update-cycle.ps1` | `.github/workflows/windows.yml` |
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

Result: `bin/Tetiva-<version>-macos-universal.dmg` — ready to distribute — and
`bin/Tetiva-<version>-macos-universal.zip`, the archive the app downloads for an in-app update.
The script staples the app inside the zip and unpacks the zip once more to run the check the app
runs before installing: `codesign --verify --deep --strict -R=<requirement>`, where the requirement
comes from `go run ./build/updatesign requirement`.

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
cross-compiles on Ubuntu with `build/release-windows.sh`, then on Windows x64 and ARM64 installs
1.2.0 from S3, installs the new build over it and opens the app; the `update-cycle` job runs a
full in-app update (see "In-app updates" below).

Without CI: `bash build/release-windows.sh` (needs `makensis`: `brew install nsis`). It writes
unsigned `bin/Tetiva-X.Y.Z-windows-{amd64,arm64}-installer.exe`. `TAGS` (default `production`)
sets the Go build tags and `OUT_SUFFIX` is appended to the file names.

### Code signing

SignPath Foundation signs the builds (organization `Tetiva [OSS]`, project `client`). The
workflow runs `release-windows.sh exes`, sends both `client-<arch>.exe` to SignPath, packs the
signed ones with `release-windows.sh installers` and sends the two installers. Pushes to `main`
sign with the self-signed `test-signing` certificate. A release needs a manual run with `release`
checked (`gh workflow run windows.yml --ref main -f release=true`): it signs with
`release-signing`, and each of the two requests waits for an approval in SignPath.
`bash build/fetch-ci-artifacts.sh windows` takes only the installers of such a run.

- The API token is the `SIGNPATH_API_TOKEN` secret of the `signpath` environment, open to `main` only.
- The artifact configuration is kept in SignPath: a zip with exactly two `.exe`, product name
  `Tetiva`, product version from the `version` parameter.
- `build/windows/info.json` must keep the `FileVersion` string: without it SignPath fails with
  "unexpected product name ''". Wails does not generate it, so `update:build-assets` drops it.
- The uninstaller that NSIS writes during installation is not signed.

The installer is per user: `%LOCALAPPDATA%\Programs\Tetiva`, uninstall key under HKCU, no UAC
prompt. `WAILS_INSTALL_SCOPE` defaults to `user` in `project.nsi`, so the release script, the
Taskfile and CI build the same installer. Behaviour worth knowing:

- An interactive install with Tetiva running asks to close it (Retry/Cancel).
- A machine-wide copy from 1.2.0 or older (HKLM `Saveliy YudinTetiva` or `Saveliy LudinTetiva`)
  is removed through its own uninstaller, which costs one UAC prompt. If the user declines, the
  install still finishes and the last page says the old copy can be removed in Settings → Apps.
- `/S /UPDATE /D=<dir>` is how the app starts it: no migration, wait up to 60 s for
  `client.exe` to unlock (exit code 2 if it doesn't), install into `<dir>`, start Tetiva again.
- Uninstalling keeps `%APPDATA%\client.exe`: that is the WebView2 profile with the frontend
  settings.

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
carries it; CI runs the same check. `bash build/set-version.sh X.Y.Z` writes the version into all
of them, adds a `## [vX.Y.Z]` heading to `CHANGELOG.md` if there is none, and runs the check.

## In-app updates

The app reads `https://api.tetiva.app/updates/latest.json` (`?v=<version>&os=<GOOS>`, at most once
a day). Clients up to 1.2.0 read only its top-level `version` and `url`; 1.2.1 and newer ignore
them and trust only `payload`, a base64 JSON with the version, `inAppDisabled` and one artifact per
platform (macOS zip, Windows amd64 and arm64 installers, each with URL, size and sha256), signed
with ed25519. The public keys are compiled in (`internal/infrastructure/appupdate/config_release.go`).

macOS and Windows download the artifact into `<data dir>/updates/<version>/`, check size and
sha256 (macOS also unpacks the zip and runs `codesign` with the requirement above) and install on
Restart to update. Linux only reads the version and shows the apt command. Only release builds
update themselves: without the `production` tag, from an ad-hoc signed bundle, from a DMG or
App Translocation, from a read-only folder or from a Windows copy without `uninstall.exe` next to
it, Settings → Updates says why and offers the site.

Files in `<data dir>/updates/`: `<version>/` with the artifact and its `manifest.json`, `attempt`
(the version being installed; if the next start still runs an older version, the install failed),
`restore.json` (tabs to reopen) and `apply.log` (output of the macOS swap script).

### `build/updatesign`

Run from the client root:

```bash
go run ./build/updatesign sign --version X.Y.Z --dir <upload dir> --out latest.json
go run ./build/updatesign verify --manifest latest.json --files <upload dir>
go run ./build/updatesign verify --manifest latest.json --remote
go run ./build/updatesign requirement
```

`sign` hashes whichever of the three artifacts are in `--dir` and builds their S3 URLs
(`--base-url` changes the prefix). `--legacy-version V` keeps the top-level `version` at V so old
clients see nothing; `--disable 1.2.1,…` lists client versions that must not install in-app and
get the site link instead. `verify` uses the app's own parser and keys; `--files` re-hashes local
files, `--remote` downloads every URL. The release steps are in the workspace `RELEASES.md`.

The private key lives only in the login keychain (service `tetiva-update-signing`, account
`tetiva`, created with `-T ""`, so every read asks for the password) and in the owner's password
manager. `keygen` creates it once, prints the public key as Go source and rewrites
`internal/infrastructure/appupdate/testdata/release-signed.json`; a new key means a new constant
in `releaseKeys` (`config_release.go`) shipped before the switch, and signatures from both keys for
as long as older clients matter. `appupdate.Sign` takes several keys; `updatesign sign` signs with
one today.

### Testing an update locally

The `updatetest` build tag swaps in the test key from `build/updatesign/testdata/`, reads the
manifest URL and an auto-apply flag from `<data dir>/updatetest.json`
(`{"manifestUrl": "http://127.0.0.1:8765/latest.json", "autoApply": true}`; a file because `open`
drops the environment on macOS) and allows plain http to `127.0.0.1`. Never ship it: the Windows CI
and `build/release-macos.sh` fail if a release binary contains the test key.

macOS needs two Developer ID signed builds. Build A:

```bash
wails3 task darwin:package EXTRA_TAGS=updatetest
codesign --force --deep --options runtime --timestamp \
  --sign "Developer ID Application: Saveliy Ludin (KB5J57CKFL)" bin/client.app
```

Build B the same way in a scratch copy of the tree after `bash build/set-version.sh 99.0.0`, then
in that copy:

```bash
mkdir -p served && ditto -c -k --keepParent bin/client.app served/Tetiva-99.0.0-macos-universal.zip
go run ./build/updatesign sign --version 99.0.0 --dir served \
  --key-file build/updatesign/testdata/test-key.b64 --base-url http://127.0.0.1:8765/ --out served/latest.json
python3 -m http.server 8765 --bind 127.0.0.1 --directory served
```

Copy A into a writable folder, write `updatetest.json` into a fresh `TETIVA_DATA_DIR` and start A
with that variable: it should download B, swap the bundle, relaunch as 99.0.0 and remove
`updates/attempt`.

Windows runs the same cycle in CI: the `build` job makes A (`TAGS=production,updatetest`) and B
(99.0.0 from a copy of the tree) and a manifest signed with the test key, and `update-cycle.ps1`
installs A into a folder with a space and Cyrillic letters, serves B from `127.0.0.1:8765` and
waits for HKCU `DisplayVersion` 99.0.0, B's `client.exe` running and no second installer
download. It then reinstalls A, breaks one byte of the payload and checks that nothing installs.
