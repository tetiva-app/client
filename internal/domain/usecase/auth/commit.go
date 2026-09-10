package auth

import (
	"context"
	"errors"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Commit budget: every store call gets its own bounded context so a jammed store
// cannot outlive a flow manager's join deadline, well short of busy_timeout(5000).
const (
	commitAttempt = 1500 * time.Millisecond
	commitBackoff = 200 * time.Millisecond
)

// commitToken writes through the store's generation guard: losing the race to another writer of the
// SAME configuration adopts that token, so ErrStaleToken means the row was cleared, re-keyed or swept.
func commitToken(ctx context.Context, repo TokenRepository, owner entities.AuthOwner,
	generation int64, hash string, tok *Token, now time.Time,
) (*Token, error) {
	var lastErr error

	for attempt := range 2 {
		if attempt > 0 {
			timer := time.NewTimer(commitBackoff)
			select {
			case <-ctx.Done():
				timer.Stop()

				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		putCtx, cancel := context.WithTimeout(ctx, commitAttempt)
		err := repo.Put(putCtx, owner, generation, hash, tok)
		// Read before the cancel below turns every expiry into a plain cancellation.
		outOfBudget := errors.Is(putCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
		cancel()

		switch {
		case err == nil:
			return tok, nil
		case errors.Is(err, ErrStaleToken):
			return adoptStoredToken(ctx, repo, owner, hash, now)
		case errors.Is(err, ErrStoreBusy), outOfBudget:
			lastErr = err
		default:
			return nil, err
		}
	}

	return nil, lastErr
}

// adoptStoredToken answers the benign lost race: the row already holds a usable token for the same
// configuration. A failed re-read proves nothing, so the original staleness stands.
func adoptStoredToken(ctx context.Context, repo TokenRepository, owner entities.AuthOwner,
	hash string, now time.Time,
) (*Token, error) {
	// The re-read is part of the commit, so it carries the commit budget too.
	getCtx, cancel := context.WithTimeout(ctx, commitAttempt)
	defer cancel()

	stored, err := repo.Get(getCtx, owner)
	if err != nil || stored == nil || stored.ConfigHash != hash || !tokenUsable(stored.Token, now) {
		return nil, ErrStaleToken
	}
	adopted := stored.Token

	return &adopted, nil
}

func tokenUsable(t Token, now time.Time) bool {
	if t.AccessToken == "" {
		return false
	}

	return t.ExpiresAt.IsZero() || t.ExpiresAt.After(now.Add(refreshSkew))
}
