package wails

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// AuthService backs the OAuth 2.0 block of the Auth tab: token status, explicit acquisition
// and clearing, the browser flows, plus the inherit walk a read-only status needs.
type AuthService struct {
	provider    auth.Provider
	flows       auth.FlowManager
	sink        *AuthFlowEventSink
	requests    request.Repository
	collections request.CollectionReader
	resolver    request.AuthResolver
	vars        request.EnvironmentResolver
}

func NewAuthService(
	provider auth.Provider,
	flows auth.FlowManager,
	sink *AuthFlowEventSink,
	requests request.Repository,
	collections request.CollectionReader,
	resolver request.AuthResolver,
	vars request.EnvironmentResolver,
) *AuthService {
	return &AuthService{
		provider:    provider,
		flows:       flows,
		sink:        sink,
		requests:    requests,
		collections: collections,
		resolver:    resolver,
		vars:        vars,
	}
}

// SetEventEmitter wires the Wails event system into the flow sink (called in main.go).
func (s *AuthService) SetEventEmitter(fn func(name string, data any)) {
	s.sink.SetEmit(fn)
}

// TokenStatus reports the stored token for the configuration in the editor.
func (s *AuthService) TokenStatus(req dto.AuthConfigRequest) Result[dto.TokenStatusDTO] {
	const funcName = "AuthService.TokenStatus"
	ctx := context.Background()

	// A non-oauth2 owner simply has no token; this status line is passive and
	// must not turn a type switch into an error.
	if entities.AuthType(req.AuthType) != entities.AuthTypeOAuth2 {
		return OK(dto.TokenStatusDTO{State: auth.TokenStateNone})
	}

	owner, cfg, err := s.ownerConfig(ctx, req)
	if err != nil {
		return Err[dto.TokenStatusDTO](err)
	}
	status, err := s.provider.Status(ctx, owner, cfg)
	if err != nil {
		return Err[dto.TokenStatusDTO](fmt.Errorf("%s: %w", funcName, err))
	}

	return OK(toTokenStatusDTO(status))
}

// FetchToken acquires a token for the editor's configuration ("Get token").
func (s *AuthService) FetchToken(req dto.AuthConfigRequest) Result[dto.TokenStatusDTO] {
	const funcName = "AuthService.FetchToken"
	ctx := context.Background()

	if entities.AuthType(req.AuthType) != entities.AuthTypeOAuth2 {
		return Err[dto.TokenStatusDTO](&domain.ValidationError{Fields: map[string]string{
			"authType": "tokens can only be fetched for OAuth 2.0",
		}})
	}

	owner, cfg, err := s.ownerConfig(ctx, req)
	if err != nil {
		return Err[dto.TokenStatusDTO](err)
	}
	if _, err := s.provider.Fetch(ctx, owner, cfg); err != nil {
		return Err[dto.TokenStatusDTO](flowError(owner, err))
	}
	status, err := s.provider.Status(ctx, owner, cfg)
	if err != nil {
		return Err[dto.TokenStatusDTO](fmt.Errorf("%s: %w", funcName, err))
	}

	return OK(toTokenStatusDTO(status))
}

// ClearToken tombstones the owner's token; it accepts any auth type so a user
// who just switched away from OAuth 2.0 can still drop what was stored.
func (s *AuthService) ClearToken(req dto.AuthConfigRequest) Result[Empty] {
	const funcName = "AuthService.ClearToken"
	ctx := context.Background()

	owner, err := s.owner(ctx, req.OwnerKind, req.OwnerID)
	if err != nil {
		return Err[Empty](err)
	}
	if err := s.provider.Clear(ctx, owner); err != nil {
		return Err[Empty](fmt.Errorf("%s: %w", funcName, err))
	}

	return OK(Empty{})
}

// ResolveOwner walks the inherit chain so a request set to inherit can show the
// collection's auth read-only, and address its token by the same owner key.
func (s *AuthService) ResolveOwner(req dto.ResolveOwnerRequest) Result[dto.ResolvedOwnerDTO] {
	const funcName = "AuthService.ResolveOwner"
	ctx := context.Background()

	id, err := uuid.Parse(req.RequestID)
	if err != nil {
		return Err[dto.ResolvedOwnerDTO](&domain.ValidationError{Fields: map[string]string{
			"requestId": "invalid UUID",
		}})
	}
	r, err := s.requests.GetByID(ctx, id)
	if err != nil {
		return Err[dto.ResolvedOwnerDTO](fmt.Errorf("%s: %w", funcName, err))
	}
	if r == nil {
		return Err[dto.ResolvedOwnerDTO](&domain.NotFoundError{Entity: "request", ID: req.RequestID})
	}

	ra, err := s.resolver.ResolveAuth(ctx, r)
	if err != nil {
		return Err[dto.ResolvedOwnerDTO](fmt.Errorf("%s: %w", funcName, err))
	}

	out := dto.ResolvedOwnerDTO{AuthType: string(ra.Type), AuthData: ra.Data}
	if ra.Owner.Kind != "" {
		out.OwnerKind = ra.Owner.Kind
		out.OwnerID = ra.Owner.ID.String()
	}

	return OK(out)
}

// StartAuthCodeFlow returns the authorize URL the frontend hands to openExternal;
// the outcome arrives as an auth:flow:<flowId> event.
func (s *AuthService) StartAuthCodeFlow(req dto.StartFlowRequest) Result[dto.FlowInfoDTO] {
	return s.startFlow(req, auth.GrantAuthorizationCode)
}

// StartDeviceFlow requests the device authorization, so the user code is already
// in the answer.
func (s *AuthService) StartDeviceFlow(req dto.StartFlowRequest) Result[dto.FlowInfoDTO] {
	return s.startFlow(req, auth.GrantDeviceCode)
}

// FlowStatus answers with an empty state for an id the manager never had or no
// longer retains, so a reloaded page can tell "gone" from "still pending".
func (s *AuthService) FlowStatus(req dto.FlowRequest) Result[dto.FlowStatusDTO] {
	if _, err := uuid.Parse(req.FlowID); err != nil {
		return Err[dto.FlowStatusDTO](&domain.ValidationError{Fields: map[string]string{
			"flowId": "invalid UUID",
		}})
	}

	st, ok := s.flows.Status(req.FlowID)
	if !ok {
		return OK(dto.FlowStatusDTO{})
	}
	out := dto.FlowStatusDTO{State: string(st.State), Info: toFlowInfoDTO(st.Info)}
	if st.Err != nil {
		out.Error = st.Err.Error()
	}

	return OK(out)
}

// CancelFlow is idempotent: an unknown or already finished id is a success.
func (s *AuthService) CancelFlow(req dto.FlowRequest) Result[Empty] {
	if _, err := uuid.Parse(req.FlowID); err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{
			"flowId": "invalid UUID",
		}})
	}
	if err := s.flows.Cancel(req.FlowID); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

func (s *AuthService) startFlow(req dto.StartFlowRequest, grant string) Result[dto.FlowInfoDTO] {
	ctx := context.Background()

	if entities.AuthType(req.AuthType) != entities.AuthTypeOAuth2 {
		return Err[dto.FlowInfoDTO](&domain.ValidationError{Fields: map[string]string{
			"authType": "browser flows only exist for OAuth 2.0",
		}})
	}
	if _, err := uuid.Parse(req.FlowID); err != nil {
		return Err[dto.FlowInfoDTO](&domain.ValidationError{Fields: map[string]string{
			"flowId": "invalid UUID",
		}})
	}
	owner, fields, err := s.ownerFields(ctx, req.OwnerKind, req.OwnerID, req.AuthData)
	if err != nil {
		return Err[dto.FlowInfoDTO](err)
	}
	cfg := auth.OAuth2ConfigFromFields(fields)

	var info auth.FlowInfo
	if grant == auth.GrantAuthorizationCode {
		// redirectPort is a presentation field, so it comes from the substituted
		// document rather than from the config the hash is taken over.
		info, err = s.flows.StartAuthCode(ctx, req.FlowID, owner, cfg, fields.Str("redirectPort"))
	} else {
		info, err = s.flows.StartDevice(ctx, req.FlowID, owner, cfg)
	}
	if err != nil {
		return Err[dto.FlowInfoDTO](flowError(owner, err))
	}

	return OK(toFlowInfoDTO(info))
}

// ownerConfig resolves the owner and builds its OAuth 2.0 configuration through
// the same pipeline prepareHTTP uses (parse, substitute, read the acquisition
// fields), so the config hash matches the one Execute stores tokens under.
func (s *AuthService) ownerConfig(ctx context.Context, req dto.AuthConfigRequest) (entities.AuthOwner, auth.OAuth2Config, error) {
	owner, fields, err := s.ownerFields(ctx, req.OwnerKind, req.OwnerID, req.AuthData)
	if err != nil {
		return entities.AuthOwner{}, auth.OAuth2Config{}, err
	}

	return owner, auth.OAuth2ConfigFromFields(fields), nil
}

// ownerFields stops one step earlier than ownerConfig, at the substituted document: the
// flows need presentation fields such as redirectPort, which the config hash must not see.
func (s *AuthService) ownerFields(ctx context.Context, kind, ownerID, authData string) (entities.AuthOwner, auth.Fields, error) {
	const funcName = "AuthService.ownerFields"

	owner, err := s.owner(ctx, kind, ownerID)
	if err != nil {
		return entities.AuthOwner{}, nil, err
	}
	fields, err := auth.ParseFields(authData)
	if err != nil {
		return entities.AuthOwner{}, nil, &domain.ValidationError{Fields: map[string]string{
			"authData": "must be a JSON object",
		}}
	}
	vars, err := s.vars.ResolveVariables(ctx, owner.WorkspaceID)
	if err != nil {
		return entities.AuthOwner{}, nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return owner, auth.Substitute(fields, vars), nil
}

// owner derives the token owner key from the editor's ids; the workspace always
// comes from the owning collection, never from the frontend.
func (s *AuthService) owner(ctx context.Context, kind, idStr string) (entities.AuthOwner, error) {
	const funcName = "AuthService.owner"

	id, err := uuid.Parse(idStr)
	if err != nil {
		return entities.AuthOwner{}, &domain.ValidationError{Fields: map[string]string{
			"ownerId": "invalid UUID",
		}}
	}

	switch kind {
	case entities.AuthOwnerKindRequest:
		req, err := s.requests.GetByID(ctx, id)
		if err != nil {
			return entities.AuthOwner{}, fmt.Errorf("%s: %w", funcName, err)
		}
		if req == nil {
			return entities.AuthOwner{}, &domain.NotFoundError{Entity: "request", ID: idStr}
		}
		coll, err := s.collections.GetByID(ctx, req.CollectionID)
		if err != nil {
			return entities.AuthOwner{}, fmt.Errorf("%s: %w", funcName, err)
		}
		if coll == nil {
			return entities.AuthOwner{}, &domain.NotFoundError{Entity: "collection", ID: req.CollectionID.String()}
		}

		return entities.AuthOwner{WorkspaceID: coll.WorkspaceID, Kind: entities.AuthOwnerKindRequest, ID: req.ID}, nil

	case entities.AuthOwnerKindCollection:
		coll, err := s.collections.GetByID(ctx, id)
		if err != nil {
			return entities.AuthOwner{}, fmt.Errorf("%s: %w", funcName, err)
		}
		if coll == nil {
			return entities.AuthOwner{}, &domain.NotFoundError{Entity: "collection", ID: idStr}
		}

		return entities.AuthOwner{WorkspaceID: coll.WorkspaceID, Kind: entities.AuthOwnerKindCollection, ID: coll.ID}, nil

	default:
		return entities.AuthOwner{}, &domain.ValidationError{Fields: map[string]string{
			"ownerKind": "must be request or collection",
		}}
	}
}

// flowError turns the auth sentinels into messages the Auth tab can show; a
// ValidationError from the flow manager passes through with its field keys.
func flowError(owner entities.AuthOwner, err error) error {
	switch {
	case errors.Is(err, auth.ErrTokenRequired):
		return &domain.ValidationError{Fields: map[string]string{
			"auth": "press Get token in the Auth tab to run the browser flow",
		}}
	case errors.Is(err, auth.ErrFlowCancelled):
		return &domain.ValidationError{Fields: map[string]string{
			"auth": "the token request was cancelled",
		}}
	case errors.Is(err, auth.ErrManagerClosed):
		return &domain.ValidationError{Fields: map[string]string{
			"auth": "the app is shutting down; try again after restarting it",
		}}
	case errors.Is(err, auth.ErrOwnerGone):
		return &domain.NotFoundError{Entity: owner.Kind, ID: owner.ID.String()}
	case errors.Is(err, auth.ErrStaleToken):
		return &domain.ValidationError{
			Fields: map[string]string{"auth": "the token was cleared while it was being fetched; try again"},
		}
	}

	return err
}

func toTokenStatusDTO(s auth.TokenStatus) dto.TokenStatusDTO {
	out := dto.TokenStatusDTO{State: s.State}
	if !s.ExpiresAt.IsZero() {
		out.ExpiresAt = s.ExpiresAt.UTC().Format(time.RFC3339)
	}

	return out
}

func toFlowInfoDTO(i auth.FlowInfo) dto.FlowInfoDTO {
	out := dto.FlowInfoDTO{
		FlowID:                  i.ID,
		AuthorizeURL:            i.AuthorizeURL,
		UserCode:                i.UserCode,
		VerificationURI:         i.VerificationURI,
		VerificationURIComplete: i.VerificationURIComplete,
		IntervalSec:             int(i.Interval / time.Second),
	}
	if !i.ExpiresAt.IsZero() {
		out.ExpiresAt = i.ExpiresAt.UTC().Format(time.RFC3339)
	}

	return out
}
