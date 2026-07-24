// Package settings defines the generic key-value app settings usecase.
package settings

import "context"

// Repository persists settings as key-value strings. ISP — usecase owns the contract.
type Repository interface {
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Set(ctx context.Context, key, value string) error
}

// Usecase is the public API. Typed accessors live in the per-domain files
// (e.g. mcp.go) and wrap the generic KV repository.
type Usecase interface {
	GetMCPConfig(ctx context.Context) (MCPConfig, error)
	SetMCPConfig(ctx context.Context, cfg MCPConfig) error
}

type usecase struct {
	repo Repository
}

// NewUsecase creates a new settings usecase instance.
func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}
