package app

import (
	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/adapters/requester"
	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/history"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/search"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/internal/infrastructure/scriptengine"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// NewUsecases provides repositories, usecases, and Wails services.
func NewUsecases() fx.Option {
	return fx.Module("usecases",
		fx.Provide(fx.Annotate(sqlite.NewCollectionRepo, fx.ResultTags(`name:"innerCollectionRepo"`))),
		fx.Provide(fx.Annotate(
			syncsvc.NewSyncedCollectionRepo,
			fx.ParamTags(`name:"innerCollectionRepo"`, ``, ``, ``),
			fx.As(new(collection.Repository)),
		)),
		fx.Provide(collection.NewUsecase),
		fx.Provide(wailsadapter.NewCollectionService),

		fx.Provide(fx.Annotate(sqlite.NewEnvironmentRepo, fx.ResultTags(`name:"innerEnvironmentRepo"`))),
		fx.Provide(fx.Annotate(
			syncsvc.NewSyncedEnvironmentRepo,
			fx.ParamTags(`name:"innerEnvironmentRepo"`, ``, ``, ``),
			fx.As(new(environment.Repository)),
		)),

		fx.Provide(fx.Annotate(sqlite.NewVariableRepo, fx.ResultTags(`name:"innerVariableRepo"`))),
		fx.Provide(fx.Annotate(
			syncsvc.NewSyncedVariableRepo,
			fx.ParamTags(`name:"innerVariableRepo"`, ``, ``, ``),
			fx.As(new(environment.VariableRepository)),
		)),

		fx.Provide(environment.NewUsecase),
		fx.Provide(wailsadapter.NewEnvironmentService),
		fx.Provide(func(eu environment.Usecase) request.EnvironmentResolver { return eu }),
		fx.Provide(scriptengine.NewGojaEngine),
		fx.Provide(func(e *scriptengine.GojaEngine) request.ScriptEngine { return e }),
		fx.Provide(NewCollectionReaderAdapter),
		fx.Provide(request.NewScriptResolver),
		fx.Provide(request.NewAuthResolver),
		fx.Provide(func(eu environment.Usecase) request.VariablePersister { return eu }),

		fx.Provide(fx.Annotate(sqlite.NewRequestRepo, fx.ResultTags(`name:"innerRequestRepo"`))),
		fx.Provide(fx.Annotate(
			syncsvc.NewSyncedRequestRepo,
			fx.ParamTags(`name:"innerRequestRepo"`, ``, ``, ``),
			fx.As(new(request.Repository)),
		)),

		fx.Provide(sqlite.NewHistoryRepo),
		fx.Provide(func(r *sqlite.HistoryRepo) request.HistoryRepository { return r }),
		fx.Provide(func(r *sqlite.HistoryRepo) history.Repository { return r }),
		fx.Provide(history.NewUsecase),
		fx.Provide(wailsadapter.NewHistoryService),

		fx.Provide(sqlite.NewCookieRepo),
		fx.Provide(cookie.NewUsecase),
		fx.Provide(requester.NewCookieStore),

		fx.Provide(requester.NewHTTPRequester),
		fx.Provide(func(r *requester.HTTPRequester) request.HTTPRequester { return r }),
		fx.Provide(func(r *requester.HTTPRequester) request.CookieReader { return r }),
		fx.Provide(requester.NewGRPCRequester),
		fx.Provide(func(r *requester.GRPCRequester) request.GRPCRequester { return r }),
		fx.Provide(requester.NewGraphQLRequester),
		fx.Provide(func(r *requester.GraphQLRequester) request.GraphQLRequester { return r }),
		fx.Provide(request.NewUsecase),
		fx.Provide(wailsadapter.NewRequestService),
		fx.Provide(wailsadapter.NewPortabilityService),
		fx.Provide(sqlite.NewWorkspaceRepo),
		fx.Provide(workspace.NewUsecase),
		fx.Provide(wailsadapter.NewWorkspaceService),
		// Search (read-only, no sync layer needed)
		fx.Provide(sqlite.NewSearchRepo),
		fx.Provide(search.NewUsecase),
		fx.Provide(wailsadapter.NewSearchService),
		fx.Provide(wailsadapter.NewCookieService),
		fx.Provide(sqlite.NewSettingsRepo),
		fx.Provide(settings.NewUsecase),
		fx.Provide(wailsadapter.NewSettingsService),
		fx.Provide(wailsadapter.NewWindowService),

		fx.Invoke(RegisterCleanupHook),
	)
}
