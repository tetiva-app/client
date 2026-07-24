package request

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// ResolveWebSocket loads the request and returns the final handshake URL and
// headers, applying env-var substitution and auth (no scripts, no body).
func (u *usecase) ResolveWebSocket(ctx context.Context, requestID, workspaceID uuid.UUID, _ string) (string, map[string][]string, error) {
	const funcName = "request.ResolveWebSocket"

	req, err := u.repo.GetByID(ctx, requestID)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return "", nil, &domain.NotFoundError{Entity: "request", ID: requestID.String()}
	}
	if req.Protocol != entities.ProtocolWebSocket {
		return "", nil, &domain.ValidationError{Fields: map[string]string{"protocol": "not a websocket request"}}
	}

	vars, err := u.envResolver.ResolveVariables(ctx, workspaceID)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", funcName, err)
	}

	headers := entities.EnabledHeadersToMap(req.Headers)
	execURL := substituteVariables(req.URL, vars)
	headers = substituteHeaders(headers, vars)

	// Enforce ws:// or wss:// — coder/websocket.Dial would otherwise accept
	// http(s):// schemes, contradicting the WebSocket protocol (spec).
	if !strings.HasPrefix(execURL, "ws://") && !strings.HasPrefix(execURL, "wss://") {
		return "", nil, &domain.ValidationError{Fields: map[string]string{"url": "must start with ws:// or wss://"}}
	}

	resolvedAuthType, resolvedAuthData, authErr := u.authResolver.ResolveAuth(ctx, req)
	if authErr != nil {
		return "", nil, fmt.Errorf("%s: %w", funcName, authErr)
	}
	if resolvedAuthData != "" && resolvedAuthData != "{}" {
		resolvedAuthData = substituteVariables(resolvedAuthData, vars)
	}
	headers, execURL, err = applyAuth(resolvedAuthType, resolvedAuthData, headers, execURL)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return execURL, headers, nil
}
