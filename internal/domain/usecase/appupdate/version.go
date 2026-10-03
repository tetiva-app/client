package appupdate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Platform struct{ OS, Arch string }

func Newer(a, b string) bool {
	av, ok := parseVersion(a)
	if !ok {
		return false
	}
	bv, ok := parseVersion(b)
	if !ok {
		return false
	}
	return slices.Compare(av[:], bv[:]) > 0
}

func ValidVersion(v string) bool {
	_, ok := parseVersion(v)
	return ok
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if strings.Trim(p, "0123456789") != "" {
			return out, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func PickArtifact(r entities.Release, p Platform) (entities.Artifact, bool) {
	for _, a := range r.Artifacts {
		if a.OS != p.OS {
			continue
		}
		switch p.OS {
		case "darwin":
			if a.Arch == "universal" && a.Format == "zip" {
				return a, true
			}
		case "windows":
			if a.Arch == p.Arch && a.Format == "nsis" {
				return a, true
			}
		}
	}
	return entities.Artifact{}, false
}
