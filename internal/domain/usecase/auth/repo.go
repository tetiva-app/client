package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// ErrStaleToken is returned by Put when the owner's generation moved between the
// Reserve that opened the acquisition and the write.
var ErrStaleToken = errors.New("auth: token generation changed")

// ErrOwnerGone is returned by Reserve when the owner row is missing, deleted or
// no longer lives in the workspace the caller named.
var ErrOwnerGone = errors.New("auth: owner no longer exists")

// ErrStoreBusy is a write the database refused because another connection held
// the lock past busy_timeout. It is retryable; ErrStaleToken is not.
var ErrStoreBusy = errors.New("auth: token store busy")

// TokenRepository keeps one row per owner. The table never syncs and never exports, and rows are
// physically deleted — the exception to soft delete, because a cleared token must leave the file.
type TokenRepository interface {
	// Get returns nil when the owner has no row or only a tombstone.
	Get(ctx context.Context, owner entities.AuthOwner) (*StoredToken, error)
	// Reserve runs before any network I/O: it makes sure a row exists for a live owner (an empty one
	// when there was none, tokens untouched otherwise) and returns the generation the Put must quote.
	Reserve(ctx context.Context, owner entities.AuthOwner) (int64, error)
	// Put fails with ErrStaleToken when the generation moved since Reserve and advances it on success,
	// so two writers that reserved the same value cannot both commit. ErrStoreBusy is retryable.
	Put(ctx context.Context, owner entities.AuthOwner, expectedGeneration int64, hash string, t *Token) error
	// Clear leaves a tombstone: blank tokens and a bumped generation, also when
	// no row existed, so a first fetch already in flight cannot write back.
	Clear(ctx context.Context, owner entities.AuthOwner) error
	// ClearOwners tombstones the rows of the given owners in every workspace.
	ClearOwners(ctx context.Context, kind string, ids []uuid.UUID) error
	// ClearOwnersUnlessHash does the same, but spares a row holding a token acquired with keepHash —
	// the edit saved the configuration that made it. An empty keepHash spares nothing.
	ClearOwnersUnlessHash(ctx context.Context, kind string, ids []uuid.UUID, keepHash string) error
	// DeleteOrphans blanks and then drops every row whose owner is gone, soft-deleted, under a deleted
	// collection, moved to another workspace or no longer oauth2; an in-flight Put then hits ErrStaleToken.
	DeleteOrphans(ctx context.Context) (int, error)
	// Generation returns the owner's current generation, 0 when it has no row.
	Generation(ctx context.Context, owner entities.AuthOwner) (int64, error)
}
