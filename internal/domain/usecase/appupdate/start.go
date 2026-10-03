package appupdate

import (
	"context"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func (u *usecase) Start(context.Context) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.store.Cleanup()
	st := entities.UpdateState{Phase: entities.UpdateIdle, Current: u.current}
	if u.restoreStaged() {
		st = u.releaseState(entities.UpdateReady, "")
	}
	// An attempt newer than this build means the installer never replaced it.
	if v, ok := u.store.TakeAttempt(); ok && Newer(v, u.current) {
		if st.Phase == entities.UpdateReady {
			st.Reason = ReasonInstallFailed
		} else {
			st = entities.UpdateState{Phase: entities.UpdateError, Version: v, Current: u.current, Reason: ReasonInstallFailed}
		}
	}
	u.set(st)
}

func (u *usecase) restoreStaged() bool {
	raw, file, ok := u.store.Staged()
	if !ok {
		return false
	}
	if rel, err := u.codec.Parse(raw); err == nil && Newer(rel.Version, u.current) {
		if a, ok := PickArtifact(rel, u.platform); ok && u.stagedFor(file, rel, a) {
			u.release, u.raw, u.artifact = &rel, raw, a
			return true
		}
	}
	u.dropStaged()
	return false
}
