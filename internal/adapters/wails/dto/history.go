package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// ListHistoryRequest is the frontend request to list history records with filtering.
type ListHistoryRequest struct {
	WorkspaceID string   `json:"workspaceId"`
	RequestID   string   `json:"requestId,omitempty"`
	Protocols   []string `json:"protocols,omitempty"`
	StatusKinds []string `json:"statusKinds,omitempty"`
	URLContains string   `json:"urlContains,omitempty"`
	Limit       int      `json:"limit"`
	Offset      int      `json:"offset"`
}

// ListHistoryResponse is the frontend response with the page of items + total count.
type ListHistoryResponse struct {
	Items      []HistoryRecord `json:"items"`
	TotalCount int             `json:"totalCount"`
}

// DeleteHistoryRequest is the frontend request to delete a single history record.
type DeleteHistoryRequest struct {
	HistoryID   string `json:"historyId"`
	WorkspaceID string `json:"workspaceId"`
}

// ClearHistoryRequest is the frontend request to clear all history in a workspace.
type ClearHistoryRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

// ReplayHistoryRequest is the frontend request to replay a history record as a draft.
type ReplayHistoryRequest struct {
	HistoryID   string `json:"historyId"`
	WorkspaceID string `json:"workspaceId"`
}

// HistoryRecord is the DTO representation of a domain History entity.
type HistoryRecord struct {
	ID              string              `json:"id"`
	RequestID       string              `json:"requestId,omitempty"`
	WorkspaceID     string              `json:"workspaceId"`
	Protocol        string              `json:"protocol"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	RequestHeaders  map[string][]string `json:"requestHeaders"`
	RequestBody     string              `json:"requestBody"`
	ResponseStatus  int                 `json:"responseStatus"`
	ResponseHeaders map[string][]string `json:"responseHeaders"`
	ResponseBody    string              `json:"responseBody"`
	ResponseSize    int64               `json:"responseSize"`
	DurationMs      int64               `json:"durationMs"`
	ErrorMessage    string              `json:"errorMessage,omitempty"`
	CreatedAt       string              `json:"createdAt"`
}

// HistoryToRecord maps a domain History entity to a HistoryRecord DTO.
func HistoryToRecord(h *entities.History) HistoryRecord {
	var requestID string
	if h.RequestID != uuid.Nil {
		requestID = h.RequestID.String()
	}
	requestHeaders := h.RequestHeaders
	if requestHeaders == nil {
		requestHeaders = make(map[string][]string)
	}
	responseHeaders := h.ResponseHeaders
	if responseHeaders == nil {
		responseHeaders = make(map[string][]string)
	}
	return HistoryRecord{
		ID:              h.ID.String(),
		RequestID:       requestID,
		WorkspaceID:     h.WorkspaceID.String(),
		Protocol:        string(h.Protocol),
		Method:          h.Method,
		URL:             h.URL,
		RequestHeaders:  requestHeaders,
		RequestBody:     h.RequestBody,
		ResponseStatus:  h.ResponseStatus,
		ResponseHeaders: responseHeaders,
		ResponseBody:    h.ResponseBody,
		ResponseSize:    h.ResponseSize,
		DurationMs:      h.DurationMs,
		ErrorMessage:    h.ErrorMessage,
		CreatedAt:       h.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// HistoryToRecords maps a slice of domain History entities to HistoryRecord DTOs.
func HistoryToRecords(items []*entities.History) []HistoryRecord {
	out := make([]HistoryRecord, 0, len(items))
	for _, h := range items {
		out = append(out, HistoryToRecord(h))
	}
	return out
}
