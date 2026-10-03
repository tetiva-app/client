package appupdate

import (
	"testing"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.2.2", "1.2.1", true},
		{"1.10.0", "1.9.9", true},
		{"1.2.1", "1.2.1", false},
		{"1.2.0", "1.2.1", false},
		{"1.2.2-rc1", "1.2.1", false},
		{"v1.2.2", "1.2.1", false},
		{"", "1.2.1", false},
		{"99.0.0", "1.2.1", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestValidVersion(t *testing.T) {
	cases := map[string]bool{
		"1.2.3":   true,
		"1.2":     false,
		"1.2.3.4": false,
		"1.2.3 ":  false,
	}
	for v, want := range cases {
		if got := ValidVersion(v); got != want {
			t.Errorf("ValidVersion(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestPickArtifact(t *testing.T) {
	mac := entities.Artifact{OS: "darwin", Arch: "universal", Format: "zip", URL: "mac"}
	winAMD := entities.Artifact{OS: "windows", Arch: "amd64", Format: "nsis", URL: "win-amd64"}
	winARM := entities.Artifact{OS: "windows", Arch: "arm64", Format: "nsis", URL: "win-arm64"}
	msi := entities.Artifact{OS: "windows", Arch: "amd64", Format: "msi", URL: "msi"}
	bsd := entities.Artifact{OS: "freebsd", Arch: "amd64", Format: "nsis", URL: "bsd"}
	rel := entities.Release{Version: "1.2.2", Artifacts: []entities.Artifact{msi, bsd, mac, winAMD, winARM}}

	cases := []struct {
		name string
		rel  entities.Release
		p    Platform
		want string
		ok   bool
	}{
		{"darwin arm64", rel, Platform{"darwin", "arm64"}, "mac", true},
		{"darwin amd64", rel, Platform{"darwin", "amd64"}, "mac", true},
		{"windows amd64", rel, Platform{"windows", "amd64"}, "win-amd64", true},
		{"windows arm64", rel, Platform{"windows", "arm64"}, "win-arm64", true},
		{"freebsd", rel, Platform{"freebsd", "amd64"}, "", false},
		{"linux", rel, Platform{"linux", "amd64"}, "", false},
		{"msi only", entities.Release{Artifacts: []entities.Artifact{msi}}, Platform{"windows", "amd64"}, "", false},
		{"first match wins", entities.Release{Artifacts: []entities.Artifact{
			winAMD, {OS: "windows", Arch: "amd64", Format: "nsis", URL: "second"},
		}}, Platform{"windows", "amd64"}, "win-amd64", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, ok := PickArtifact(c.rel, c.p)
			if ok != c.ok || a.URL != c.want {
				t.Errorf("PickArtifact = (%q, %v), want (%q, %v)", a.URL, ok, c.want, c.ok)
			}
		})
	}
}
