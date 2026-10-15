package dto

import (
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type CreateExampleRequest struct {
	RequestID   string          `json:"requestId"`
	Name        string          `json:"name"`
	StatusCode  int             `json:"statusCode"`
	StatusText  string          `json:"statusText"`
	Headers     []HeaderItemDTO `json:"headers"`
	Body        string          `json:"body"`
	ContentType string          `json:"contentType"`
	Protocol    string          `json:"protocol"`
}

type EditExampleRequest struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	StatusCode  int             `json:"statusCode"`
	StatusText  string          `json:"statusText"`
	Headers     []HeaderItemDTO `json:"headers"`
	Body        string          `json:"body"`
	ContentType string          `json:"contentType"`
	Version     int             `json:"version"`
}

type DeleteExampleRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type ExampleResponse struct {
	ID          string          `json:"id"`
	RequestID   string          `json:"requestId"`
	Name        string          `json:"name"`
	StatusCode  int             `json:"statusCode"`
	StatusText  string          `json:"statusText"`
	Headers     []HeaderItemDTO `json:"headers"`
	Body        string          `json:"body"`
	ContentType string          `json:"contentType"`
	Protocol    string          `json:"protocol"`
	SortOrder   int             `json:"sortOrder"`
	Version     int             `json:"version"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

func ExampleToResponse(e *entities.ResponseExample) ExampleResponse {
	return ExampleResponse{
		ID:          e.ID.String(),
		RequestID:   e.RequestID.String(),
		Name:        e.Name,
		StatusCode:  e.StatusCode,
		StatusText:  e.StatusText,
		Headers:     HeaderItemsToDTO(e.Headers),
		Body:        e.Body,
		ContentType: e.ContentType,
		Protocol:    string(e.Protocol),
		SortOrder:   e.SortOrder,
		Version:     e.Version,
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ExamplesToResponse(examples []*entities.ResponseExample) []ExampleResponse {
	result := make([]ExampleResponse, 0, len(examples))
	for _, e := range examples {
		result = append(result, ExampleToResponse(e))
	}
	return result
}

type ScanExampleRequest struct {
	Headers []HeaderItemDTO `json:"headers"`
	Body    string          `json:"body"`
}
