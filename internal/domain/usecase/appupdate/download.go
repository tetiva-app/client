package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

const progressInterval = 250 * time.Millisecond

func (u *usecase) Download(ctx context.Context) entities.UpdateState {
	u.mu.Lock()
	if u.state.Phase != entities.UpdateAvailable || u.state.Install != entities.InstallInApp {
		defer u.mu.Unlock()
		return u.state
	}
	rel, raw, a := *u.release, u.raw, u.artifact
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	u.cancel = cancel
	st := u.releaseState(entities.UpdateDownloading, "")
	st.Total = a.Size
	u.set(st)
	u.mu.Unlock()

	reason, err := u.stage(ctx, dctx, rel, raw, a)

	u.mu.Lock()
	defer u.mu.Unlock()
	u.cancel = nil
	switch {
	case err == nil:
		u.set(u.releaseState(entities.UpdateReady, ""))
	case dctx.Err() != nil:
		u.set(u.releaseState(entities.UpdateAvailable, ""))
	default:
		slog.Warn("appupdate: download failed", "version", rel.Version, "reason", reason, "err", err)
		u.set(u.releaseState(entities.UpdateError, reason))
	}
	return u.state
}

func (u *usecase) Cancel() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.cancel != nil {
		u.cancel()
	}
}

func (u *usecase) stage(ctx, dctx context.Context, rel entities.Release, raw []byte, a entities.Artifact) (string, error) {
	const funcName = "appupdate.stage"
	file := u.store.StagePath(rel, a)
	part := file + ".part"
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return ReasonInstallFailed, fmt.Errorf("%s: %w", funcName, err)
	}
	if err := u.dl.Download(dctx, a, part, u.progress()); err != nil {
		_ = os.Remove(part)
		return reasonOf(err, ReasonNetwork), fmt.Errorf("%s: %w", funcName, err)
	}
	if err := fileMatches(part, a); err != nil {
		_ = os.Remove(part)
		return ReasonChecksum, fmt.Errorf("%s: %w", funcName, err)
	}
	if err := os.Rename(part, file); err != nil {
		_ = os.Remove(part)
		return ReasonInstallFailed, fmt.Errorf("%s: %w", funcName, err)
	}
	if err := u.inst.Verify(ctx, rel, file); err != nil {
		_ = os.Remove(file)
		return reasonOf(err, ReasonCodesignFailed), fmt.Errorf("%s: %w", funcName, err)
	}
	if err := u.store.SaveStaged(rel, raw, file); err != nil {
		_ = os.Remove(file)
		return ReasonInstallFailed, fmt.Errorf("%s: %w", funcName, err)
	}
	return "", nil
}

func (u *usecase) progress() func(received int64) {
	last := time.Now()
	return func(received int64) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.state.Received = received
		if time.Since(last) >= progressInterval {
			last = time.Now()
			u.sink.Publish(u.state)
		}
	}
}

func fileMatches(file string, a entities.Artifact) error {
	const funcName = "appupdate.fileMatches"
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if n != a.Size || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return fmt.Errorf("%s: %s does not match the signed size and sha256", funcName, filepath.Base(file))
	}
	return nil
}

func reasonOf(err error, fallback string) string {
	var re *domain.ReasonError
	if errors.As(err, &re) {
		return re.Reason
	}
	return fallback
}
