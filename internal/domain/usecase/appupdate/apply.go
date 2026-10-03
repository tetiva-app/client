package appupdate

import (
	"context"
	"fmt"
	"time"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

const applyCheckTimeout = 5 * time.Second

func (u *usecase) Apply(ctx context.Context, restore entities.RestoreTabs) error {
	const funcName = "appupdate.Apply"
	// A staged copy may predate inAppDisabled or a rollback; offline it installs as is.
	fetchCtx, cancel := context.WithTimeout(ctx, applyCheckTimeout)
	u.check(ctx, fetchCtx)
	cancel()

	u.mu.Lock()
	if u.state.Phase != entities.UpdateReady {
		phase := u.state.Phase
		u.mu.Unlock()
		return fmt.Errorf("%s: no update is ready (phase %s)", funcName, phase)
	}
	rel, a := *u.release, u.artifact
	u.set(u.releaseState(entities.UpdateApplying, ""))
	u.mu.Unlock()

	if reason, err := u.install(ctx, rel, a, restore); err != nil {
		u.mu.Lock()
		u.set(u.releaseState(entities.UpdateError, reason))
		u.mu.Unlock()
		return fmt.Errorf("%s: %w", funcName, &domain.ReasonError{Reason: reason, Err: err})
	}
	return nil
}

func (u *usecase) install(ctx context.Context, rel entities.Release, a entities.Artifact, restore entities.RestoreTabs) (string, error) {
	file := u.store.StagePath(rel, a)
	if err := fileMatches(file, a); err != nil {
		u.dropStaged()
		return ReasonChecksum, err
	}
	err := u.store.SaveRestore(restore)
	if err == nil {
		err = u.store.MarkAttempt(rel.Version)
	}
	if err == nil {
		err = u.inst.Apply(ctx, rel, file)
	}
	if err != nil {
		u.store.TakeRestore()
		u.store.TakeAttempt()
		return ReasonInstallFailed, err
	}
	return "", nil
}
