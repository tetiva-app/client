//go:build darwin

package appupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

type darwinInstaller struct{ updatesDir string }

func newPlatformInstaller(dataDir string) appupdate.Installer {
	return darwinInstaller{updatesDir: filepath.Join(dataDir, "updates")}
}

func (d darwinInstaller) Kind(ctx context.Context) (entities.InstallKind, string) {
	bundle, err := runningBundle()
	if err != nil {
		slog.Warn("appupdate: not running from an app bundle", "err", err)
		return entities.InstallUnsupported, appupdate.ReasonNotInstalledCopy
	}
	if err := runTool(ctx, "codesign", "--verify", "-R="+CodeRequirement, bundle); err != nil {
		// Exit 3: a valid signature that fails -R, which is what ad-hoc signed dev bundles have.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
			return entities.InstallUnsupported, appupdate.ReasonDevBuild
		}
		slog.Warn("appupdate: codesign rejected the running bundle", "err", err)
		return entities.InstallUnsupported, appupdate.ReasonCodesignFailed
	}
	return darwinKindFromPath(bundle, func(p string) bool { return unix.Access(p, unix.W_OK) == nil })
}

func (d darwinInstaller) Verify(ctx context.Context, r entities.Release, file string) error {
	const funcName = "appupdate.darwinInstaller.Verify"
	tmp, err := os.MkdirTemp(d.updatesDir, "verify-")
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := runTool(ctx, "ditto", "-x", "-k", file, tmp); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if _, err := verifyBundle(ctx, tmp, r.Version); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (d darwinInstaller) Apply(ctx context.Context, r entities.Release, file string) error {
	const funcName = "appupdate.darwinInstaller.Apply"
	bundle, err := runningBundle()
	if err != nil {
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	logFile, err := os.OpenFile(filepath.Join(d.updatesDir, "apply.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	defer func() { _ = logFile.Close() }()
	// RENAME_SWAP works only within one volume, so the new bundle is unpacked next to the old one.
	side, err := os.MkdirTemp(filepath.Dir(bundle), ".tetiva-update-")
	if err != nil {
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	if err := swapBundle(ctx, file, side, bundle, r.Version); err != nil {
		_ = os.RemoveAll(side)
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	cmd := exec.Command("/bin/sh", "-c", relaunchScript(), "sh",
		strconv.Itoa(os.Getpid()), bundle, side, os.Getenv("TETIVA_DATA_DIR"))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		return reasonError(funcName, appupdate.ReasonInstallFailed, err)
	}
	return nil
}

func swapBundle(ctx context.Context, file, side, bundle, version string) error {
	if err := runTool(ctx, "ditto", "-x", "-k", file, side); err != nil {
		return err
	}
	app, err := verifyBundle(ctx, side, version)
	if err != nil {
		return err
	}
	return unix.RenamexNp(app, bundle, unix.RENAME_SWAP)
}

func verifyBundle(ctx context.Context, dir, version string) (string, error) {
	const funcName = "appupdate.verifyBundle"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", reasonError(funcName, appupdate.ReasonCodesignFailed, err)
	}
	var apps []string
	for _, e := range entries {
		if e.IsDir() && filepath.Ext(e.Name()) == ".app" {
			apps = append(apps, filepath.Join(dir, e.Name()))
		}
	}
	if len(apps) != 1 {
		return "", reasonError(funcName, appupdate.ReasonCodesignFailed, fmt.Errorf("want one .app in the archive, found %d", len(apps)))
	}
	app := apps[0]
	if err := runTool(ctx, "codesign", "--verify", "--deep", "--strict", "-R="+CodeRequirement, app); err != nil {
		return "", reasonError(funcName, appupdate.ReasonCodesignFailed, err)
	}
	out, err := exec.CommandContext(ctx, "plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-",
		filepath.Join(app, "Contents", "Info.plist")).Output()
	if err != nil {
		return "", reasonError(funcName, appupdate.ReasonCodesignFailed, err)
	}
	if got := strings.TrimSpace(string(out)); got != version {
		return "", reasonError(funcName, appupdate.ReasonCodesignFailed, fmt.Errorf("bundle is %q, manifest says %q", got, version))
	}
	return app, nil
}

func runningBundle() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	bundle, ok := bundleFromExecutable(exe)
	if !ok {
		return "", fmt.Errorf("%s is not inside an app bundle", exe)
	}
	return bundle, nil
}

func runTool(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, bytes.TrimSpace(out))
	}
	return nil
}
