package appupdate

import (
	"path"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

// CodeRequirement admits only Developer ID builds of our bundle id signed by our team.
const CodeRequirement = `identifier "yudinsv.com.Tetiva" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] and certificate leaf[field.1.2.840.113635.100.6.1.13] and certificate leaf[subject.OU] = "KB5J57CKFL"`

// path, not filepath: these are always macOS paths, and tests run on every OS.
func bundleFromExecutable(exe string) (string, bool) {
	macOS := path.Dir(exe)
	contents := path.Dir(macOS)
	bundle := path.Dir(contents)
	if path.Base(macOS) != "MacOS" || path.Base(contents) != "Contents" || path.Ext(bundle) != ".app" {
		return "", false
	}
	return bundle, true
}

func darwinKindFromPath(bundle string, writable func(string) bool) (entities.InstallKind, string) {
	if strings.Contains(bundle, "/AppTranslocation/") || strings.HasPrefix(bundle, "/Volumes/") {
		return entities.InstallUnsupported, appupdate.ReasonTranslocated
	}
	// The swap renames inside the parent, so both have to be writable.
	if !writable(bundle) || !writable(path.Dir(bundle)) {
		return entities.InstallUnsupported, appupdate.ReasonReadOnly
	}
	return entities.InstallInApp, ""
}

func relaunchScript() string {
	return `i=0
while kill -0 "$1" 2>/dev/null; do
  i=$((i+1)); [ "$i" -gt 120 ] && exit 1
  sleep 0.5
done
if [ -n "$4" ]; then open -n "$2" --env "TETIVA_DATA_DIR=$4"; else open -n "$2"; fi
rm -rf "$3"
`
}
