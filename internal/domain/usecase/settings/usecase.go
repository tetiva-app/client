// Package settings defines the generic key-value app settings usecase.
package settings

import "context"

type Repository interface {
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Set(ctx context.Context, key, value string) error
	// SetMany commits every key or none: a config split across keys must not
	// survive a restart half applied.
	SetMany(ctx context.Context, values map[string]string) error
}

// Typed accessors live in the per-domain files (e.g. mcp.go) and wrap the generic KV repository.
type Usecase interface {
	GetMCPConfig(ctx context.Context) (MCPConfig, error)
	SetMCPConfig(ctx context.Context, cfg MCPConfig) error
	// Token accessors are separate from MCPConfig: SetMCPConfig writes the whole
	// config, so a UI save would otherwise wipe the token.
	GetMCPToken(ctx context.Context) (string, error)
	EnsureMCPToken(ctx context.Context) (string, error)
	RegenerateMCPToken(ctx context.Context) (string, error)
	GetMCPRequireToken(ctx context.Context) (bool, error)
}

type usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}
