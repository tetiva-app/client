package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/adapters/requester"
	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

func WebSocketModule() fx.Option {
	return fx.Module("websocket",
		fx.Provide(requester.NewWebSocketRequester),
		fx.Provide(func(r *requester.WebSocketRequester) websocket.Dialer { return r }),
		fx.Provide(wailsadapter.NewWebSocketEventSink),
		fx.Provide(func(s *wailsadapter.WebSocketEventSink) websocket.MessageSink { return s }),
		fx.Provide(func(ru request.Usecase) websocket.RequestResolver { return ru }),
		// Reuse the same concrete history repo the request usecase uses.
		fx.Provide(func(r request.HistoryRepository) websocket.HistoryRepository { return r }),
		fx.Provide(websocket.NewUsecase),
		fx.Provide(wailsadapter.NewWebSocketService),
		// WebSocketService itself is constructed via main.go's fx.Populate;
		// this shutdown hook only needs the usecase.
		fx.Invoke(func(lc fx.Lifecycle, uc websocket.Usecase) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := uc.DisconnectAll(ctx); err != nil {
						slog.Warn("websocket: DisconnectAll on shutdown failed", "err", err)
					}
					return nil
				},
			})
		}),
	)
}
