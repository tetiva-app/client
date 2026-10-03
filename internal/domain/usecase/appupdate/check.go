package appupdate

import (
	"context"
	"log/slog"
	"slices"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func (u *usecase) Check(ctx context.Context) entities.UpdateState {
	return u.check(ctx, ctx)
}

func (u *usecase) check(ctx, fetchCtx context.Context) entities.UpdateState {
	u.mu.Lock()
	if u.busy() {
		defer u.mu.Unlock()
		return u.state
	}
	// Ready stays shown while checking, so an offline launch still offers the restart.
	if u.state.Phase != entities.UpdateReady {
		u.set(entities.UpdateState{Phase: entities.UpdateChecking, Current: u.current})
	}
	u.mu.Unlock()

	kind, kindReason := u.inst.Kind(ctx)
	failReason := ReasonNetwork
	raw, err := u.src.Fetch(fetchCtx)
	var rel entities.Release
	if err == nil {
		failReason = ReasonBadManifest
		rel, err = u.codec.Parse(raw)
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if u.busy() {
		return u.state
	}
	if err != nil {
		slog.Warn("appupdate: check failed", "reason", failReason, "err", err)
		if u.state.Phase != entities.UpdateReady {
			u.set(entities.UpdateState{Phase: entities.UpdateError, Current: u.current, Reason: failReason})
		}
		return u.state
	}
	if !Newer(rel.Version, u.current) {
		u.dropStaged()
		u.set(entities.UpdateState{Phase: entities.UpdateUpToDate, Current: u.current})
		return u.state
	}

	u.release, u.raw = &rel, raw
	st := entities.UpdateState{
		Phase:   entities.UpdateAvailable,
		Version: rel.Version,
		Current: u.current,
		Install: kind,
		Reason:  kindReason,
	}
	switch {
	case kind != entities.InstallInApp:
	case slices.Contains(rel.InAppDisabled, u.current):
		u.dropStaged()
		st.Install, st.Reason = entities.InstallUnsupported, ReasonDisabled
	default:
		a, ok := PickArtifact(rel, u.platform)
		if !ok {
			st.Install, st.Reason = entities.InstallUnsupported, ReasonNoArtifact
			break
		}
		u.artifact = a
		if _, file, staged := u.store.Staged(); staged {
			if u.stagedFor(file, rel, a) {
				st.Phase = entities.UpdateReady
				if u.state.Phase == entities.UpdateReady && u.state.Version == rel.Version {
					st.Reason = u.state.Reason
				}
			} else {
				u.dropStaged()
			}
		}
	}
	u.set(st)
	return u.state
}

func (u *usecase) stagedFor(file string, rel entities.Release, a entities.Artifact) bool {
	return file == u.store.StagePath(rel, a) && fileMatches(file, a) == nil
}

func (u *usecase) dropStaged() {
	if err := u.store.DropStaged(); err != nil {
		slog.Warn("appupdate: failed to drop the staged update", "err", err)
	}
}
