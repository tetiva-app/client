package wails

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/history"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// stubHistoryUsecase is a configurable history.Usecase double; unset function
// fields panic to surface accidental misuse.
type stubHistoryUsecase struct {
	listFn    func(context.Context, history.ListOpt) ([]*entities.History, int, error)
	getByIDFn func(context.Context, uuid.UUID, uuid.UUID) (*entities.History, error)
	deleteFn  func(context.Context, history.DeleteOpt) error
	clearFn   func(context.Context, history.ClearOpt) error
}

func (s *stubHistoryUsecase) List(ctx context.Context, opt history.ListOpt) ([]*entities.History, int, error) {
	if s.listFn == nil {
		panic("listFn not set")
	}
	return s.listFn(ctx, opt)
}

func (s *stubHistoryUsecase) GetByID(ctx context.Context, id, ws uuid.UUID) (*entities.History, error) {
	if s.getByIDFn == nil {
		panic("getByIDFn not set")
	}
	return s.getByIDFn(ctx, id, ws)
}

func (s *stubHistoryUsecase) Delete(ctx context.Context, opt history.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubHistoryUsecase) Clear(ctx context.Context, opt history.ClearOpt) error {
	if s.clearFn == nil {
		panic("clearFn not set")
	}
	return s.clearFn(ctx, opt)
}

func fixtureHistory(ws uuid.UUID) *entities.History {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.History{
		ID:              uuid.New(),
		RequestID:       uuid.New(),
		WorkspaceID:     ws,
		Protocol:        entities.ProtocolHTTP,
		Method:          "GET",
		URL:             "https://api.example.com/x",
		RequestHeaders:  map[string][]string{"X-Test": {"1"}},
		RequestBody:     "",
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
		ResponseBody:    `{"ok":true}`,
		ResponseSize:    11,
		DurationMs:      42,
		CreatedAt:       now,
	}
}

func TestHistoryService_List_Happy(t *testing.T) {
	ws := uuid.New()
	h := fixtureHistory(ws)
	uc := &stubHistoryUsecase{
		listFn: func(_ context.Context, opt history.ListOpt) ([]*entities.History, int, error) {
			assert.Equal(t, ws, opt.WorkspaceID)
			assert.Equal(t, 10, opt.Filter.Limit)
			return []*entities.History{h}, 1, nil
		},
	}
	svc := NewHistoryService(uc, nil)

	res := svc.List(dto.ListHistoryRequest{WorkspaceID: ws.String(), Limit: 10})

	require.Nil(t, res.Error)
	assert.Equal(t, 1, res.Data.TotalCount)
	require.Len(t, res.Data.Items, 1)
	assert.Equal(t, "GET", res.Data.Items[0].Method)
}

func TestHistoryService_List_Error_BadWorkspaceUUID(t *testing.T) {
	svc := NewHistoryService(&stubHistoryUsecase{}, nil)

	res := svc.List(dto.ListHistoryRequest{WorkspaceID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestHistoryService_GetByID_Happy(t *testing.T) {
	ws := uuid.New()
	id := uuid.New()
	h := fixtureHistory(ws)
	h.ID = id
	uc := &stubHistoryUsecase{
		getByIDFn: func(_ context.Context, gotID, gotWS uuid.UUID) (*entities.History, error) {
			assert.Equal(t, id, gotID)
			assert.Equal(t, ws, gotWS)
			return h, nil
		},
	}
	svc := NewHistoryService(uc, nil)

	res := svc.GetByID(id.String(), ws.String())

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, ws.String(), res.Data.WorkspaceID)
}

func TestHistoryService_GetByID_Error_BadHistoryUUID(t *testing.T) {
	svc := NewHistoryService(&stubHistoryUsecase{}, nil)

	res := svc.GetByID("not-a-uuid", uuid.NewString())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "historyId")
}

func TestHistoryService_Delete_Happy(t *testing.T) {
	ws := uuid.New()
	id := uuid.New()
	uc := &stubHistoryUsecase{
		deleteFn: func(_ context.Context, opt history.DeleteOpt) error {
			assert.Equal(t, id, opt.HistoryID)
			assert.Equal(t, ws, opt.WorkspaceID)
			return nil
		},
	}
	svc := NewHistoryService(uc, nil)

	res := svc.Delete(dto.DeleteHistoryRequest{HistoryID: id.String(), WorkspaceID: ws.String()})

	require.Nil(t, res.Error)
}

func TestHistoryService_Delete_Error_BadHistoryUUID(t *testing.T) {
	svc := NewHistoryService(&stubHistoryUsecase{}, nil)

	res := svc.Delete(dto.DeleteHistoryRequest{HistoryID: "not-a-uuid", WorkspaceID: uuid.NewString()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "historyId")
}

func TestHistoryService_Clear_Happy(t *testing.T) {
	ws := uuid.New()
	uc := &stubHistoryUsecase{
		clearFn: func(_ context.Context, opt history.ClearOpt) error {
			assert.Equal(t, ws, opt.WorkspaceID)
			return nil
		},
	}
	svc := NewHistoryService(uc, nil)

	res := svc.Clear(dto.ClearHistoryRequest{WorkspaceID: ws.String()})

	require.Nil(t, res.Error)
}

func TestHistoryService_Clear_Error_BadWorkspaceUUID(t *testing.T) {
	svc := NewHistoryService(&stubHistoryUsecase{}, nil)

	res := svc.Clear(dto.ClearHistoryRequest{WorkspaceID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestHistoryService_Replay_Happy(t *testing.T) {
	ws := uuid.New()
	historyID := uuid.New()
	draft := fixtureRequest("replayed", 1)
	reqStub := &stubRequestUsecase{
		createDraftFromHistoryFn: func(_ context.Context, opt request.CreateDraftFromHistoryOpt) (*entities.Request, error) {
			assert.Equal(t, historyID, opt.HistoryID)
			assert.Equal(t, ws, opt.WorkspaceID)
			assert.Equal(t, "local_user", opt.UserID)
			return draft, nil
		},
	}
	svc := NewHistoryService(&stubHistoryUsecase{}, reqStub)

	res := svc.Replay(dto.ReplayHistoryRequest{HistoryID: historyID.String(), WorkspaceID: ws.String()})

	require.Nil(t, res.Error)
	assert.NotEmpty(t, res.Data.ID)
	assert.Equal(t, "replayed", res.Data.Name)
}

func TestHistoryService_Replay_Error_BadHistoryUUID(t *testing.T) {
	svc := NewHistoryService(&stubHistoryUsecase{}, &stubRequestUsecase{})

	res := svc.Replay(dto.ReplayHistoryRequest{HistoryID: "not-a-uuid", WorkspaceID: uuid.NewString()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "historyId")
}

func TestHistoryService_Replay_Error_UsecasePropagates(t *testing.T) {
	ws := uuid.New()
	historyID := uuid.New()
	reqStub := &stubRequestUsecase{
		createDraftFromHistoryFn: func(_ context.Context, _ request.CreateDraftFromHistoryOpt) (*entities.Request, error) {
			return nil, &domain.NotFoundError{Entity: "history", ID: historyID.String()}
		},
	}
	svc := NewHistoryService(&stubHistoryUsecase{}, reqStub)

	res := svc.Replay(dto.ReplayHistoryRequest{HistoryID: historyID.String(), WorkspaceID: ws.String()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestHistoryService_List_Error_UsecasePropagates(t *testing.T) {
	ws := uuid.New()
	uc := &stubHistoryUsecase{
		listFn: func(_ context.Context, _ history.ListOpt) ([]*entities.History, int, error) {
			return nil, 0, errors.New("boom")
		},
	}
	svc := NewHistoryService(uc, nil)

	res := svc.List(dto.ListHistoryRequest{WorkspaceID: ws.String(), Limit: 10})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}
