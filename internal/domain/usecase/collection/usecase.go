package collection

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Repository interface {
	Create(ctx context.Context, c *entities.Collection) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
	List(ctx context.Context, filter Filter) ([]*entities.Collection, error)
	Update(ctx context.Context, c *entities.Collection) error
	UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error
	SoftDeleteDescendants(ctx context.Context, parentID uuid.UUID, updatedBy string, updatedAt time.Time) error
}

// TokenCleaner invalidates OAuth 2.0 tokens owned by a collection or anything under it.
type TokenCleaner interface {
	Clear(ctx context.Context, owner entities.AuthOwner) error
	ClearOwnersUnlessHash(ctx context.Context, kind string, ids []uuid.UUID, keepHash string) error
	DeleteOrphans(ctx context.Context) (int, error)
}

// noopTokenCleaner stands in for test constructors built without a token store.
type noopTokenCleaner struct{}

func (noopTokenCleaner) Clear(context.Context, entities.AuthOwner) error { return nil }

func (noopTokenCleaner) ClearOwnersUnlessHash(context.Context, string, []uuid.UUID, string) error {
	return nil
}

func (noopTokenCleaner) DeleteOrphans(context.Context) (int, error) { return 0, nil }

type TxRunner interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

// directTx stands in for test constructors built without a database: fn runs without atomicity.
type directTx struct{}

func (directTx) Run(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// PublicationMarker records, inside the delete transaction, that the collection's public page has to come down.
type PublicationMarker interface {
	MarkPendingUnpublish(ctx context.Context, collectionIDs []uuid.UUID) error
}

type noopPublicationMarker struct{}

func (noopPublicationMarker) MarkPendingUnpublish(context.Context, []uuid.UUID) error { return nil }

type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Collection, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
	List(ctx context.Context, opt ListOpt) ([]*entities.Collection, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Collection, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error
	Move(ctx context.Context, opt MoveOpt) (*entities.Collection, error)
}

type usecase struct {
	repo         Repository
	tokenCleaner TokenCleaner
	txRunner     TxRunner
	publications PublicationMarker
}

func NewUsecase(repo Repository, tokenCleaner TokenCleaner, tx TxRunner, publications PublicationMarker) Usecase {
	return &usecase{repo: repo, tokenCleaner: tokenCleaner, txRunner: tx, publications: publications}
}

// tokens returns the injected cleaner, or a no-op for usecases built without one.
func (u *usecase) tokens() TokenCleaner {
	if u.tokenCleaner == nil {
		return noopTokenCleaner{}
	}
	return u.tokenCleaner
}

func (u *usecase) tx() TxRunner {
	if u.txRunner == nil {
		return directTx{}
	}
	return u.txRunner
}

func (u *usecase) publicationMarker() PublicationMarker {
	if u.publications == nil {
		return noopPublicationMarker{}
	}
	return u.publications
}
