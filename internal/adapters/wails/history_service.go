package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/history"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// HistoryService exposes request-history operations to the frontend via Wails.
type HistoryService struct {
	historyUC history.Usecase
	requestUC request.Usecase
}

func NewHistoryService(historyUC history.Usecase, requestUC request.Usecase) *HistoryService {
	return &HistoryService{historyUC: historyUC, requestUC: requestUC}
}

func (s *HistoryService) List(req dto.ListHistoryRequest) Result[dto.ListHistoryResponse] {
	ctx := context.Background()
	ws, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.ListHistoryResponse](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}

	var requestIDPtr *uuid.UUID
	if req.RequestID != "" {
		rid, err := uuid.Parse(req.RequestID)
		if err != nil {
			return Err[dto.ListHistoryResponse](&domain.ValidationError{Fields: map[string]string{"requestId": "invalid UUID"}})
		}
		requestIDPtr = &rid
	}

	protocols := make([]entities.Protocol, 0, len(req.Protocols))
	for _, p := range req.Protocols {
		protocols = append(protocols, entities.Protocol(p))
	}
	kinds := make([]history.StatusKind, 0, len(req.StatusKinds))
	for _, k := range req.StatusKinds {
		kinds = append(kinds, history.StatusKind(k))
	}

	items, total, err := s.historyUC.List(ctx, history.ListOpt{
		WorkspaceID: ws,
		Filter: history.Filter{
			WorkspaceID: ws,
			RequestID:   requestIDPtr,
			Protocols:   protocols,
			StatusKinds: kinds,
			URLContains: req.URLContains,
			Limit:       req.Limit,
			Offset:      req.Offset,
		},
	})
	if err != nil {
		return Err[dto.ListHistoryResponse](err)
	}
	return OK(dto.ListHistoryResponse{Items: dto.HistoryToRecords(items), TotalCount: total})
}

func (s *HistoryService) GetByID(historyIDStr, workspaceIDStr string) Result[dto.HistoryRecord] {
	ctx := context.Background()
	id, err := uuid.Parse(historyIDStr)
	if err != nil {
		return Err[dto.HistoryRecord](&domain.ValidationError{Fields: map[string]string{"historyId": "invalid UUID"}})
	}
	ws, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[dto.HistoryRecord](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}
	h, err := s.historyUC.GetByID(ctx, id, ws)
	if err != nil {
		return Err[dto.HistoryRecord](err)
	}
	return OK(dto.HistoryToRecord(h))
}

func (s *HistoryService) Delete(req dto.DeleteHistoryRequest) Result[Empty] {
	ctx := context.Background()
	id, err := uuid.Parse(req.HistoryID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"historyId": "invalid UUID"}})
	}
	ws, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}
	if err := s.historyUC.Delete(ctx, history.DeleteOpt{HistoryID: id, WorkspaceID: ws}); err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

func (s *HistoryService) Clear(req dto.ClearHistoryRequest) Result[Empty] {
	ctx := context.Background()
	ws, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}
	if err := s.historyUC.Clear(ctx, history.ClearOpt{WorkspaceID: ws}); err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

// Creates a draft request populated from a history record.
func (s *HistoryService) Replay(req dto.ReplayHistoryRequest) Result[dto.RequestResponse] {
	ctx := context.Background()
	historyID, err := uuid.Parse(req.HistoryID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{Fields: map[string]string{"historyId": "invalid UUID"}})
	}
	ws, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}
	draft, err := s.requestUC.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: ws,
		UserID:      "local_user",
	})
	if err != nil {
		return Err[dto.RequestResponse](err)
	}
	return OK(dto.RequestToResponse(draft))
}
