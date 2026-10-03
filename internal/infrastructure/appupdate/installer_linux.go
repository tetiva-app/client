//go:build linux

package appupdate

import (
	"context"
	"errors"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

var errAPTOnly = errors.New("updates on Linux go through APT")

type linuxInstaller struct{}

func newPlatformInstaller(string) appupdate.Installer { return linuxInstaller{} }

func (linuxInstaller) Kind(context.Context) (entities.InstallKind, string) {
	if aptConfigured("/") {
		return entities.InstallAPT, ""
	}
	return entities.InstallAPTNotConfigured, ""
}

func (linuxInstaller) Verify(context.Context, entities.Release, string) error {
	return &domain.ReasonError{Reason: appupdate.ReasonNotInstalledCopy, Err: errAPTOnly}
}

func (linuxInstaller) Apply(context.Context, entities.Release, string) error {
	return &domain.ReasonError{Reason: appupdate.ReasonNotInstalledCopy, Err: errAPTOnly}
}
