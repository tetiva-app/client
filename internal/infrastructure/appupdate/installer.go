package appupdate

import (
	"context"
	"errors"
	"fmt"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

func NewInstaller(dataDir string, releaseBuild bool) appupdate.Installer {
	if !releaseBuild {
		return devInstaller{}
	}
	return newPlatformInstaller(dataDir)
}

var errDevBuild = errors.New("development builds do not update themselves")

type devInstaller struct{}

func (devInstaller) Kind(context.Context) (entities.InstallKind, string) {
	return entities.InstallUnsupported, appupdate.ReasonDevBuild
}

func (devInstaller) Verify(context.Context, entities.Release, string) error {
	return &domain.ReasonError{Reason: appupdate.ReasonDevBuild, Err: errDevBuild}
}

func (devInstaller) Apply(context.Context, entities.Release, string) error {
	return &domain.ReasonError{Reason: appupdate.ReasonDevBuild, Err: errDevBuild}
}

func reasonError(funcName, reason string, err error) error {
	return &domain.ReasonError{Reason: reason, Err: fmt.Errorf("%s: %w", funcName, err)}
}
