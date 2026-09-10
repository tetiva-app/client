package environment

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Repository interface {
	Create(ctx context.Context, e *entities.Environment) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Environment, error)
	List(ctx context.Context, filter Filter) ([]*entities.Environment, error)
	Update(ctx context.Context, e *entities.Environment) error
	GetActive(ctx context.Context, workspaceID uuid.UUID) (*entities.Environment, error)
	SetActive(ctx context.Context, workspaceID uuid.UUID, environmentID uuid.UUID) error
}

type VariableRepository interface {
	Create(ctx context.Context, v *entities.Variable) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Variable, error)
	List(ctx context.Context, environmentID uuid.UUID) ([]*entities.Variable, error)
	Update(ctx context.Context, v *entities.Variable) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Environment, error)
	Duplicate(ctx context.Context, opt DuplicateOpt) (*entities.Environment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Environment, error)
	List(ctx context.Context, opt ListOpt) ([]*entities.Environment, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Environment, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	SetActive(ctx context.Context, opt SetActiveOpt) error

	AddVariable(ctx context.Context, input AddVariable, opt AddVariableOpt) (*entities.Variable, error)
	EditVariable(ctx context.Context, input EditVariable, opt EditVariableOpt) (*entities.Variable, error)
	DeleteVariable(ctx context.Context, opt DeleteVariableOpt) error
	ListVariables(ctx context.Context, environmentID uuid.UUID) ([]*entities.Variable, error)

	ResolveVariables(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)

	PersistVariableChanges(ctx context.Context, workspaceID uuid.UUID, userID string, newVars map[string]string) error
}

type usecase struct {
	repo    Repository
	varRepo VariableRepository
}

func NewUsecase(repo Repository, varRepo VariableRepository) Usecase {
	return &usecase{repo: repo, varRepo: varRepo}
}
