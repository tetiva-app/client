package appupdate

import (
	"os"
	"path/filepath"
	"strings"
)

func aptConfigured(root string) bool {
	files := []string{filepath.Join(root, "etc", "apt", "sources.list")}
	for _, pattern := range []string{"*.list", "*.sources"} {
		matches, _ := filepath.Glob(filepath.Join(root, "etc", "apt", "sources.list.d", pattern))
		files = append(files, matches...)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, stanza := range strings.Split(string(b), "\n\n") {
			if listsTetiva(stanza) {
				return true
			}
		}
	}
	return false
}

func listsTetiva(stanza string) bool {
	found := false
	for _, line := range strings.Split(stanza, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Enabled") && strings.EqualFold(strings.TrimSpace(v), "no") {
			return false
		}
		found = found || strings.Contains(line, "apt.tetiva.app")
	}
	return found
}
