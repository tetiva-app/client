#!/usr/bin/env bash
set -euo pipefail
# Usage: smoke.sh <new.deb> [<old.deb>], as root in a clean Ubuntu container.

colors() { xwd -root -silent | od -An -v -tx4 | tr -s ' ' '\n' | sort -u | wc -l; }

if [ "${1:-}" = --open ]; then
  title=$2
  blank=$(colors)
  log=$(mktemp)
  tetiva 2>"$log" &
  app=$!
  deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    sleep 2
    kill -0 "$app" 2>/dev/null || break
    if xwininfo -root -tree | grep -qF "\"$title\"" && pgrep -f WebKitWebProcess >/dev/null &&
      [ "$(colors)" -gt $((blank + 2000)) ]; then
      kill "$app"
      exit 0
    fi
  done
  failures=()
  kill -0 "$app" 2>/dev/null || failures+=("tetiva exited")
  xwininfo -root -tree | grep -qF "\"$title\"" || failures+=("no window \"$title\"")
  pgrep -f WebKitWebProcess >/dev/null || failures+=("no WebKitWebProcess")
  now=$(colors)
  [ "$now" -gt $((blank + 2000)) ] || failures+=("nothing rendered: $now colors, $blank before launch")
  if [ ${#failures[@]} -eq 0 ]; then
    kill "$app"
    exit 0
  fi
  cat "$log"
  printf '%s\n' "${failures[@]}"
  exit 1
fi

new=$(realpath "$1")
old=${2:+$(realpath "$2")}

apt-get update -qq
apt-get install -y -qq --no-install-recommends xvfb xauth x11-utils x11-apps dbus procps >/dev/null

if [ -n "$old" ]; then
  apt-get install -y -qq "$old" >/dev/null
  have=$(dpkg-query -W -f='${Version}' tetiva)
  [ "$have" = "$(dpkg-deb -f "$old" Version)" ] || { echo "old package: installed $have"; exit 1; }
fi

# --reinstall: apt skips a same-version .deb whose control fields match the installed one.
apt-get install -y -qq --reinstall "$new" >/dev/null
fresh=$(dpkg-deb --fsys-tarfile "$new" | tar -xO ./usr/bin/tetiva | sha256sum | cut -d' ' -f1)
[ "$fresh" = "$(sha256sum /usr/bin/tetiva | cut -d' ' -f1)" ] ||
  { echo "/usr/bin/tetiva is not the binary from $new"; exit 1; }

version=$(dpkg-deb -f "$new" Version)
export HOME TETIVA_DATA_DIR GDK_BACKEND=x11 NO_AT_BRIDGE=1
HOME=$(mktemp -d)
TETIVA_DATA_DIR=$HOME
xvfb-run -a -s "-screen 0 1280x800x24" dbus-run-session -- bash "$0" --open "Tetiva ${version%-*}"
