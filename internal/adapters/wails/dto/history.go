package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type ListHistoryRequest struct {
	WorkspaceID string   `json:"workspaceId"`
	RequestID   string   `json:"requestId,omitempty"`
	Protocols   []string `json:"protocols,omitempty"`
	StatusKinds []string `json:"statusKinds,omitempty"`
	URLContains string   `json:"urlContains,omitempty"`
	Limit       int      `json:"limit"`
	Offset      int      `json:"offset"`
}

type ListHistoryResponse struct {
	Items      []HistoryRecord `json:"items"`
	TotalCount int             `json:"totalCount"`
}

type DeleteHistoryRequest struct {
	HistoryID   string `json:"historyId"`
	WorkspaceID string `json:"workspaceId"`
}

type ClearHistoryRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

type ReplayHistoryRequest struct {
	HistoryID   string `json:"historyId"`
	WorkspaceID string `json:"workspaceId"`
}

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

func HistoryToRecords(items []*entities.History) []HistoryRecord {
	out := make([]HistoryRecord, 0, len(items))
	for _, h := range items {
		out = append(out, HistoryToRecord(h))
	}
	return out
}
