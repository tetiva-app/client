package appupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallerCmdLine(t *testing.T) {
	tests := []struct {
		name, installer, installDir, want string
	}{
		{
			name:       "space",
			installer:  `C:\Users\Ivan Petrov\AppData\Local\Tetiva\updates\1.2.2\Tetiva-1.2.2-windows-amd64-installer.exe`,
			installDir: `C:\Users\Ivan Petrov\AppData\Local\Programs\Tetiva`,
			want:       `"C:\Users\Ivan Petrov\AppData\Local\Tetiva\updates\1.2.2\Tetiva-1.2.2-windows-amd64-installer.exe" /S /UPDATE /D=C:\Users\Ivan Petrov\AppData\Local\Programs\Tetiva`,
		},
		{
			name:       "cyrillic",
			installer:  `C:\Users\Иван Петров\AppData\Local\Tetiva\updates\1.2.2\Tetiva-1.2.2-windows-amd64-installer.exe`,
			installDir: `C:\Users\Иван Петров\AppData\Local\Programs\Tetiva`,
			want:       `"C:\Users\Иван Петров\AppData\Local\Tetiva\updates\1.2.2\Tetiva-1.2.2-windows-amd64-installer.exe" /S /UPDATE /D=C:\Users\Иван Петров\AppData\Local\Programs\Tetiva`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, installerCmdLine(tt.installer, tt.installDir))
		})
	}
}
