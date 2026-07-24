package request

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// CreateDraftFromHistoryOpt holds options for the CreateDraftFromHistory operation.
type CreateDraftFromHistoryOpt struct {
	HistoryID   uuid.UUID
	WorkspaceID uuid.UUID
	UserID      string
}

// ErrNoCollectionForDraft is returned when no collection exists in the workspace
// to host the draft request. Wrapped via %w by the caller.
var ErrNoCollectionForDraft = &domain.ValidationError{
	Fields: map[string]string{"workspace": "no collection available for draft"},
}

// draftURLLabelLimit caps the URL substring used in the draft name (tab label).
const draftURLLabelLimit = 60

// CreateDraftFromHistory creates a temporary Request (is_draft=1) from a history
// record. Scripts, auth, and header Enabled flags are not in history, so scripts/auth
// are dropped and headers restored as enabled.
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

	now := time.Now()
	draft := &entities.Request{
		ID:           uuid.New(),
		CollectionID: collectionID,
		Name:         draftName(h),
		Protocol:     h.Protocol,
		Method:       entities.HTTPMethod(h.Method),
		URL:          h.URL,
		Headers:      headerItemsFromMap(h.RequestHeaders),
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

// resolveDraftCollection prefers the source request's collection (if still alive),
// then the workspace's first collection; otherwise ErrNoCollectionForDraft.
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

// firstCollectionFor returns the lowest-sort_order collection in the workspace.
// CollectionReader.ListByWorkspace is expected to return results already sorted.
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

// draftName builds a short, recognisable label for a replay draft tab.
func draftName(h *entities.History) string {
	label := h.URL
	if len(label) > draftURLLabelLimit {
		label = label[:draftURLLabelLimit] + "…"
	}
	return "Replay: " + label
}

// PromoteDraftOpt holds options for converting a draft into a regular Request.
// Version must equal the draft's current version (optimistic locking).
type PromoteDraftOpt struct {
	DraftID            uuid.UUID
	Name               string
	TargetCollectionID uuid.UUID
	UserID             string
	Version            int
}

// DeleteDraft hard-deletes a draft request. Returns NotFoundError if the
// request does not exist and ValidationError if the request is not a draft.
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
	if err := u.repo.DeleteHard(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// PromoteDraft converts a draft into a regular request with the given name and
// collection. Returns NotFoundError, ValidationError (non-draft), or ConflictError.
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
	return existing, nil
}

// CleanupDrafts removes all drafts left over from a previous session and
// returns the number of rows removed. Invoked on app startup.
func (u *usecase) CleanupDrafts(ctx context.Context) (int, error) {
	const funcName = "request.CleanupDrafts"
	n, err := u.repo.CleanupDrafts(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	return n, nil
}

// headerItemsFromMap converts the history's flat header map into []HeaderItem,
// marking every entry as enabled. Keys are sorted for deterministic order.
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
