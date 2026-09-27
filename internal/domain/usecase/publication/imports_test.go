package publication

import (
	"go/build"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const modulePath = "github.com/tetiva-app/client"

func TestImportsStayInsideTheAllowlist(t *testing.T) {
	allowedInternal := map[string]bool{
		modulePath + "/internal/domain":                   true,
		modulePath + "/internal/domain/entities":          true,
		modulePath + "/internal/domain/secrets":           true,
		modulePath + "/internal/domain/usecase/auth":      true,
		modulePath + "/internal/domain/usecase/websocket": true,
	}
	forbiddenStd := map[string]bool{
		"os": true, "os/exec": true, "io/fs": true, "io/ioutil": true, "path/filepath": true,
		"net": true, "net/http": true, "database/sql": true, "embed": true,
	}

	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	pkg, err := build.ImportDir(filepath.Dir(file), 0)
	if err != nil {
		t.Fatalf("import publication: %v", err)
	}
	for _, imp := range pkg.Imports {
		switch {
		case strings.HasPrefix(imp, modulePath+"/"):
			if !allowedInternal[imp] {
				t.Errorf("publication imports %s", imp)
			}
		case !strings.Contains(strings.Split(imp, "/")[0], "."):
			if forbiddenStd[imp] {
				t.Errorf("publication imports %s", imp)
			}
		case imp != "github.com/google/uuid":
			t.Errorf("publication imports %s", imp)
		}
	}

	seen := map[string]bool{}
	var walk func(path string)
	walk = func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		if !allowedInternal[path] {
			t.Errorf("publication depends on %s", path)
			return
		}
		dep, err := build.ImportDir(filepath.Join(root, strings.TrimPrefix(path, modulePath)), 0)
		if err != nil {
			t.Fatalf("import %s: %v", path, err)
		}
		for _, imp := range dep.Imports {
			if strings.HasPrefix(imp, modulePath+"/") {
				walk(imp)
			}
		}
	}
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp, modulePath+"/") {
			walk(imp)
		}
	}
}
