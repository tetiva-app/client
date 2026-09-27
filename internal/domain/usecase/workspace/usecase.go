package workspace

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Repository interface {
	Create(ctx context.Context, w *entities.Workspace) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error)
	List(ctx context.Context) ([]*entities.Workspace, error)
	Update(ctx context.Context, w *entities.Workspace) error
	GetActive(ctx context.Context) (*entities.Workspace, error)
	SetActive(ctx context.Context, id uuid.UUID) error
	CountNonDeleted(ctx context.Context) (int, error)
	GetByRemoteID(ctx context.Context, remoteID string) (*entities.Workspace, error)
}

type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Workspace, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Workspace, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	List(ctx context.Context) ([]*entities.Workspace, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error)
	GetActive(ctx context.Context) (*entities.Workspace, error)
	SetActive(ctx context.Context, opt SetActiveOpt) error
}

type TxRunner interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

// directTx stands in for test constructors built without a database: fn runs without atomicity.
type directTx struct{}

func (directTx) Run(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// PublicationMarker records, inside the delete transaction, that the workspace's public pages have to come down.
type PublicationMarker interface {
	MarkPendingUnpublishWorkspace(ctx context.Context, workspaceID uuid.UUID) error
}

type noopPublicationMarker struct{}

func (noopPublicationMarker) MarkPendingUnpublishWorkspace(context.Context, uuid.UUID) error {
	return nil
}

type usecase struct {
	repo         Repository
	txRunner     TxRunner
	publications PublicationMarker
}

func NewUsecase(repo Repository, tx TxRunner, publications PublicationMarker) Usecase {
	return &usecase{repo: repo, txRunner: tx, publications: publications}
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
