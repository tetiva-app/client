package request

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

type CreateDraftFromHistoryOpt struct {
	HistoryID   uuid.UUID
	WorkspaceID uuid.UUID
	UserID      string
}

// ErrNoCollectionForDraft means the workspace has no collection to host the draft.
var ErrNoCollectionForDraft = &domain.ValidationError{
	Fields: map[string]string{"workspace": "no collection available for draft"},
}

// draftURLLabelLimit caps the URL substring used in the draft name (tab label).
const draftURLLabelLimit = 60

// draftStrippedHeaders never reach a replay draft: a draft can be promoted into
// a normal request and then syncs, so a computed credential must not ride along.
var draftStrippedHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
}

// draftStrippedQueryParams carry credentials by convention; the keys auth actually injected come
// from the history row, since the source request may be gone or reconfigured since the send.
var draftStrippedQueryParams = map[string]bool{
	"access_token":         true,
	"refresh_token":        true,
	"id_token":             true,
	"token":                true,
	"api_key":              true,
	"apikey":               true,
	"client_secret":        true,
	"code":                 true,
	"code_verifier":        true,
	"assertion":            true,
	"signature":            true,
	"sig":                  true,
	"x-amz-signature":      true,
	"x-amz-credential":     true,
	"x-amz-security-token": true,
}

// CreateDraftFromHistory builds a temporary request (is_draft=1): history carries no scripts, auth or
// header flags, so those are dropped or restored as enabled and computed credentials are stripped.
func (u *usecase) CreateDraftFromHistory(ctx context.Context, opt CreateDraftFromHistoryOpt) (*entities.Request, error) {
	const funcName = "request.CreateDraftFromHistory"

	h, err := u.historyRepo.GetByID(ctx, opt.HistoryID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if h == nil {
		return nil, &domain.NotFoundError{Entity: "history", ID: opt.HistoryID.String()}
	}
	if h.WorkspaceID != opt.WorkspaceID {
		return nil, &domain.ValidationError{
			Fields: map[string]string{"workspaceId": "history record belongs to a different workspace"},
		}
	}

	collectionID, err := u.resolveDraftCollection(ctx, h, opt.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	draftURL := stripDraftQuery(h.URL, h.AuthQueryKeys)

	now := time.Now()
	draft := &entities.Request{
		ID:           uuid.New(),
		CollectionID: collectionID,
		Name:         draftName(draftURL),
		Protocol:     h.Protocol,
		Method:       entities.HTTPMethod(h.Method),
		URL:          draftURL,
		Headers:      headerItemsFromMap(stripDraftHeaders(h.RequestHeaders)),
		Body:         h.RequestBody,
		// History does not record the original BodyType — fall back to "raw"
		// so the body is sent verbatim if the user replays without editing.
		BodyType: entities.BodyTypeRaw,
		AuthType: entities.AuthTypeNone,
		// auth_data and grpc_metadata have json_valid() CHECK constraints in
		// SQLite; "" is not valid JSON, so default to "{}".
		AuthData:     "{}",
		GRPCMetadata: map[string][]string{},
		IsDraft:      true,
		Version:      1,
		CreatedBy:    opt.UserID,
		CreatedAt:    now,
		UpdatedBy:    opt.UserID,
		UpdatedAt:    now,
	}

	if err := u.repo.Create(ctx, draft); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return draft, nil
}

// resolveDraftCollection prefers the source request's collection, then the workspace's first one.
func (u *usecase) resolveDraftCollection(ctx context.Context, h *entities.History, workspaceID uuid.UUID) (uuid.UUID, error) {
	if h.RequestID != uuid.Nil {
		src, err := u.repo.GetByID(ctx, h.RequestID)
		if err != nil {
			return uuid.Nil, err
		}
		if src != nil && !src.IsDelete {
			return src.CollectionID, nil
		}
	}

	if u.collectionReader == nil {
		return uuid.Nil, ErrNoCollectionForDraft
	}
	firstID, err := u.firstCollectionFor(ctx, workspaceID)
	if err != nil {
		return uuid.Nil, err
	}
	if firstID == uuid.Nil {
		return uuid.Nil, ErrNoCollectionForDraft
	}
	return firstID, nil
}

// firstCollectionFor relies on ListByWorkspace returning collections already sorted by sort_order.
func (u *usecase) firstCollectionFor(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	cols, err := u.collectionReader.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return uuid.Nil, err
	}
	if len(cols) == 0 {
		return uuid.Nil, nil
	}
	return cols[0].ID, nil
}

// stripDraftHeaders drops the headers that carry credentials.
func stripDraftHeaders(headers map[string][]string) map[string][]string {
	if len(headers) == 0 {
		return headers
	}
	out := make(map[string][]string, len(headers))
	for k, v := range headers {
		if draftStrippedHeaders[strings.ToLower(k)] {
			continue
		}
		out[k] = v
	}
	return out
}

// stripDraftQuery removes credential parameters, both by name and the ones auth injected on the
// recorded send. A URL that does not parse is left alone — this app sent it itself.
func stripDraftQuery(rawURL string, authKeys []string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
		return rawURL
	}
	injected := make(map[string]bool, len(authKeys))
	for _, k := range authKeys {
		injected[strings.ToLower(k)] = true
	}

	// The pairs are kept verbatim: a replay must reproduce the recorded send,
	// and url.Values.Encode would re-order and re-escape the whole query.
	pairs := strings.Split(parsed.RawQuery, "&")
	kept := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		name := pair
		if i := strings.IndexByte(pair, '='); i >= 0 {
			name = pair[:i]
		}
		decoded, err := url.QueryUnescape(name)
		if err != nil {
			decoded = name
		}
		lower := strings.ToLower(decoded)
		if draftStrippedQueryParams[lower] || injected[lower] {
			continue
		}
		kept = append(kept, pair)
	}
	if len(kept) == len(pairs) {
		return rawURL
	}

	parsed.RawQuery = strings.Join(kept, "&")
	return parsed.String()
}

// draftName builds a short, recognisable label for a replay draft tab.
func draftName(label string) string {
	if len(label) > draftURLLabelLimit {
		label = label[:draftURLLabelLimit] + "…"
	}
	return "Replay: " + label
}

// PromoteDraftOpt.Version must equal the draft's current version (optimistic locking).
type PromoteDraftOpt struct {
	DraftID            uuid.UUID
	Name               string
	TargetCollectionID uuid.UUID
	UserID             string
	Version            int
}

// DeleteDraft is a hard delete; a request that is not a draft is a ValidationError.
func (u *usecase) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	const funcName = "request.DeleteDraft"
	existing, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return &domain.NotFoundError{Entity: "request", ID: id.String()}
	}
	if !existing.IsDraft {
		return &domain.ValidationError{Fields: map[string]string{"id": "not a draft"}}
	}
	// Clear before the row goes: a token fetch already in flight then finds its
	// generation moved and discards what it obtained.
	if err := u.tokens().ClearOwners(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{id}); err != nil {
		return fmt.Errorf("%s: clear tokens: %w", funcName, err)
	}
	if err := u.repo.DeleteHard(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	// The draft is gone; a failed sweep is housekeeping for the next one.
	if _, err := u.tokens().DeleteOrphans(ctx); err != nil {
		slog.Warn("request: token sweep failed", "op", funcName, "err", err)
	}
	return nil
}

// PromoteDraft converts a draft into a regular request; a non-draft is a ValidationError.
func (u *usecase) PromoteDraft(ctx context.Context, opt PromoteDraftOpt) (*entities.Request, error) {
	const funcName = "request.PromoteDraft"
	existing, err := u.repo.GetByID(ctx, opt.DraftID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: opt.DraftID.String()}
	}
	if !existing.IsDraft {
		return nil, &domain.ValidationError{Fields: map[string]string{"id": "not a draft"}}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "request", ID: opt.DraftID.String()}
	}
	existing.IsDraft = false
	existing.Name = opt.Name
	existing.CollectionID = opt.TargetCollectionID
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()
	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	// Promotion can move the draft into another workspace, and the sweep drops a token row whose
	// workspace no longer matches its owner; the write is committed, so a failed sweep waits.
	if _, err := u.tokens().DeleteOrphans(ctx); err != nil {
		slog.Warn("request: token sweep failed", "op", funcName, "err", err)
	}
	return existing, nil
}

// CleanupDrafts removes drafts left over from a previous session; invoked on app startup.
func (u *usecase) CleanupDrafts(ctx context.Context) (int, error) {
	const funcName = "request.CleanupDrafts"
	n, err := u.repo.CleanupDrafts(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	// The drafts are gone from the table, so their token rows are orphans now;
	// a failed sweep is retried at the next hook or at startup.
	if _, err := u.tokens().DeleteOrphans(ctx); err != nil {
		slog.Warn("request: token sweep failed", "op", funcName, "err", err)
	}
	return n, nil
}

// headerItemsFromMap marks every entry enabled and sorts the keys for a deterministic order.
func headerItemsFromMap(m map[string][]string) []entities.HeaderItem {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]entities.HeaderItem, 0, len(m))
	for _, k := range keys {
		for _, v := range m[k] {
			items = append(items, entities.HeaderItem{Key: k, Value: v, Enabled: true})
		}
	}
	return items
}
