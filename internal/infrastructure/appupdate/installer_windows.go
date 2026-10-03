//go:build windows

package appupdate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

type windowsInstaller struct{}

func newPlatformInstaller(string) appupdate.Installer { return windowsInstaller{} }

func (windowsInstaller) Kind(context.Context) (entities.InstallKind, string) {
	exe, err := os.Executable()
	if err == nil {
		_, err = os.Stat(filepath.Join(filepath.Dir(exe), "uninstall.exe"))
	}
	if err != nil {
		return entities.InstallUnsupported, appupdate.ReasonNotInstalledCopy
	}
	return entities.InstallInApp, ""
}

// Installers are unsigned, so the manifest's signed sha256 is the only check.
func (windowsInstaller) Verify(context.Context, entities.Release, string) error { return nil }

func (windowsInstaller) Apply(_ context.Context, _ entities.Release, file string) error {
	const funcName = "appupdate.windowsInstaller.Apply"
	exe, err := os.Executable()
	if err != nil {
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	cmd := exec.Command(file)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       installerCmdLine(file, filepath.Dir(exe)),
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	return nil
}
