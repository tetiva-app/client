package appupdate

import (
	"context"
	"errors"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// ReasonPreviewBuild: an update would install the stable build over the preview.
const ReasonPreviewBuild = "preview_build"

var ErrUpdatesDisabled = errors.New("appupdate: updates are disabled in this build")

type disabled struct {
	state entities.UpdateState
}

func NewDisabled(current, reason string) Usecase {
	return disabled{state: entities.UpdateState{
		Phase:   entities.UpdateIdle,
		Current: current,
		Install: entities.InstallUnsupported,
		Reason:  reason,
	}}
}

func (d disabled) Start(context.Context) {}

func (d disabled) Status() entities.UpdateState { return d.state }

func (d disabled) Check(context.Context) entities.UpdateState { return d.state }

func (d disabled) Download(context.Context) entities.UpdateState { return d.state }

func (d disabled) Cancel() {}

func (d disabled) Apply(context.Context, entities.RestoreTabs) error { return ErrUpdatesDisabled }

func (d disabled) TakeRestore() (entities.RestoreTabs, bool) { return entities.RestoreTabs{}, false }
