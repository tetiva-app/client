package request

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// ResolveWebSocket builds the handshake input from a stored request. A failure
// after the script stage lands in ResolvedDial.Failed, not in the error.
func (u *usecase) ResolveWebSocket(ctx context.Context, requestID, workspaceID uuid.UUID, userID string) (websocket.ResolvedDial, error) {
	const funcName = "request.ResolveWebSocket"

	req, err := u.repo.GetByID(ctx, requestID)
	if err != nil {
		return websocket.ResolvedDial{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return websocket.ResolvedDial{}, &domain.NotFoundError{Entity: "request", ID: requestID.String()}
	}
	if req.Protocol != entities.ProtocolWebSocket {
		return websocket.ResolvedDial{}, &domain.ValidationError{Fields: map[string]string{"protocol": "not a websocket request"}}
	}

	resolvedAuth, err := u.resolveAuthFor(ctx, req, workspaceID)
	if err != nil {
		return websocket.ResolvedDial{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if authErr := rejectNonHTTPAuth(resolvedAuth.Type); authErr != nil {
		return websocket.ResolvedDial{}, fmt.Errorf("%s: %w", funcName, authErr)
	}

	vars, err := u.envResolver.ResolveVariables(ctx, workspaceID)
	if err != nil {
		return websocket.ResolvedDial{}, fmt.Errorf("%s: %w", funcName, err)
	}

	pre := u.runPreScriptForNonHTTP(ctx, req, vars)
	settings := websocket.ParseSettings(req.Body)
	dial := websocket.ResolvedDial{
		URL:          pre.URL,
		Headers:      pre.Headers,
		Subprotocols: settings.Subprotocols,
		PingInterval: settings.PingInterval,
		Script:       pre.ScriptResult,
	}

	if u.varPersister != nil && pre.ScriptResult != nil && len(pre.Vars) > 0 {
		if persistErr := u.varPersister.PersistVariableChanges(ctx, workspaceID, userID, pre.Vars); persistErr != nil {
			pre.ScriptResult.Errors = append(pre.ScriptResult.Errors, entities.ScriptError{
				Phase: "variable-persist", Message: persistErr.Error(),
			})
		}
	}

	// Enforce ws:// or wss:// — coder/websocket.Dial would otherwise accept
	// http(s):// schemes, contradicting the WebSocket protocol (spec).
	if !strings.HasPrefix(dial.URL, "ws://") && !strings.HasPrefix(dial.URL, "wss://") {
		dial.Failed = "url must start with ws:// or wss://"
		return dial, nil
	}

	authFields, fieldsErr := auth.ParseFields(resolvedAuth.Data)
	if fieldsErr != nil {
		dial.Failed = fieldsErr.Error()
		return dial, nil
	}
	headers, execURL, queryKeys, _, err := u.applyResolvedAuth(
		ctx, resolvedAuth, auth.Substitute(authFields, pre.Vars), dial.Headers, dial.URL, prepareOpt{UserID: userID},
	)
	if err != nil {
		dial.Failed = err.Error()
		return dial, nil
	}
	dial.Headers, dial.URL, dial.AuthQueryKeys = headers, execURL, queryKeys

	return dial, nil
}

// SubstituteMessage resolves {{variables}} at send time, so a token refreshed by
// another request is picked up without reconnecting.
func (u *usecase) SubstituteMessage(ctx context.Context, workspaceID uuid.UUID, text string) (string, error) {
	const funcName = "request.SubstituteMessage"

	vars, err := u.envResolver.ResolveVariables(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	return substituteVariables(text, vars), nil
}
