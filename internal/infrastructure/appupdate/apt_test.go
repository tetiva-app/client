package appupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tetivaDebLine = "deb [signed-by=/etc/apt/keyrings/tetiva.gpg] https://apt.tetiva.app stable main\n"

func TestAPTConfigured(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{
			name:  "list file",
			files: map[string]string{"etc/apt/sources.list.d/tetiva.list": tetivaDebLine},
			want:  true,
		},
		{
			name:  "commented out",
			files: map[string]string{"etc/apt/sources.list.d/tetiva.list": "# " + tetivaDebLine},
		},
		{
			name: "deb822 sources",
			files: map[string]string{
				"etc/apt/sources.list.d/tetiva.sources": "Types: deb\nURIs: https://apt.tetiva.app\nSuites: stable\nComponents: main\n",
			},
			want: true,
		},
		{
			name: "deb822 sources disabled",
			files: map[string]string{
				"etc/apt/sources.list.d/tetiva.sources": "Types: deb\nURIs: https://apt.tetiva.app\nSuites: stable\nComponents: main\nEnabled: no\n",
			},
		},
		{
			name: "deb822 disabled stanza next to an enabled one",
			files: map[string]string{
				"etc/apt/sources.list.d/tetiva.sources": "Types: deb-src\nURIs: https://apt.tetiva.app\nEnabled: no\n\nTypes: deb\nURIs: https://apt.tetiva.app\nSuites: stable\nEnabled: yes\n",
			},
			want: true,
		},
		{
			name: "main sources.list",
			files: map[string]string{
				"etc/apt/sources.list": "deb http://archive.ubuntu.com/ubuntu noble main\n" + tetivaDebLine,
			},
			want: true,
		},
		{
			name:  "other repositories only",
			files: map[string]string{"etc/apt/sources.list": "deb http://archive.ubuntu.com/ubuntu noble main\n"},
		},
		{
			name: "nothing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tt.files {
				p := filepath.Join(root, filepath.FromSlash(name))
				require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
				require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
			}
			assert.Equal(t, tt.want, aptConfigured(root))
		})
	}
}
