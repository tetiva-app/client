package dto

import (
	"github.com/tetiva-app/client/internal/domain/entities"
)

type HeaderItemDTO struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

func HeaderItemsToEntity(dtos []HeaderItemDTO) []entities.HeaderItem {
	result := make([]entities.HeaderItem, len(dtos))
	for i, d := range dtos {
		result[i] = entities.HeaderItem{Key: d.Key, Value: d.Value, Enabled: d.Enabled}
	}
	return result
}

func HeaderItemsToDTO(items []entities.HeaderItem) []HeaderItemDTO {
	result := make([]HeaderItemDTO, len(items))
	for i, item := range items {
		result[i] = HeaderItemDTO{Key: item.Key, Value: item.Value, Enabled: item.Enabled}
	}
	return result
}

type CreateRequestRequest struct {
	CollectionID      string              `json:"collectionId"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	Protocol          string              `json:"protocol"`
	Method            string              `json:"method"`
	URL               string              `json:"url"`
	Headers           []HeaderItemDTO     `json:"headers"`
	Body              string              `json:"body"`
	BodyType          string              `json:"bodyType"`
	AuthType          string              `json:"authType"`
	AuthData          string              `json:"authData"`
	PreScript         string              `json:"preScript"`
	PostScript        string              `json:"postScript"`
	GRPCService       string              `json:"grpcService"`
	GRPCMethod        string              `json:"grpcMethod"`
	GRPCProtoPath     string              `json:"grpcProtoPath"`
	GRPCMetadata      map[string][]string `json:"grpcMetadata"`
	GraphQLQuery      string              `json:"graphqlQuery"`
	GraphQLVariables  string              `json:"graphqlVariables"`
	GraphQLSchemaPath string              `json:"graphqlSchemaPath"`
	GraphQLOperation  string              `json:"graphqlOperation"`
}

type EditRequestRequest struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	Method            string              `json:"method"`
	URL               string              `json:"url"`
	Headers           []HeaderItemDTO     `json:"headers"`
	Body              string              `json:"body"`
	BodyType          string              `json:"bodyType"`
	AuthType          string              `json:"authType"`
	AuthData          string              `json:"authData"`
	PreScript         string              `json:"preScript"`
	PostScript        string              `json:"postScript"`
	Version           int                 `json:"version"`
	GRPCService       string              `json:"grpcService"`
	GRPCMethod        string              `json:"grpcMethod"`
	GRPCProtoPath     string              `json:"grpcProtoPath"`
	GRPCMetadata      map[string][]string `json:"grpcMetadata"`
	GraphQLQuery      string              `json:"graphqlQuery"`
	GraphQLVariables  string              `json:"graphqlVariables"`
	GraphQLSchemaPath string              `json:"graphqlSchemaPath"`
	GraphQLOperation  string              `json:"graphqlOperation"`
}

type DeleteRequestRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type ReorderRequestRequest struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sortOrder"`
}

type MoveRequestRequest struct {
	ID                 string `json:"id"`
	TargetCollectionID string `json:"targetCollectionId"`
	Version            int    `json:"version"`
}

type ExecuteRequestRequest struct {
	RequestID   string `json:"requestId"`
	WorkspaceID string `json:"workspaceId"`
}

type RequestResponse struct {
	ID                string              `json:"id"`
	CollectionID      string              `json:"collectionId"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	Protocol          string              `json:"protocol"`
	Method            string              `json:"method"`
	URL               string              `json:"url"`
	Headers           []HeaderItemDTO     `json:"headers"`
	Body              string              `json:"body"`
	BodyType          string              `json:"bodyType"`
	AuthType          string              `json:"authType"`
	AuthData          string              `json:"authData"`
	PreScript         string              `json:"preScript"`
	PostScript        string              `json:"postScript"`
	GRPCService       string              `json:"grpcService"`
	GRPCMethod        string              `json:"grpcMethod"`
	GRPCProtoPath     string              `json:"grpcProtoPath"`
	GRPCMetadata      map[string][]string `json:"grpcMetadata"`
	GraphQLQuery      string              `json:"graphqlQuery"`
	GraphQLVariables  string              `json:"graphqlVariables"`
	GraphQLSchemaPath string              `json:"graphqlSchemaPath"`
	GraphQLOperation  string              `json:"graphqlOperation"`
	SortOrder         int                 `json:"sortOrder"`
	IsDraft           bool                `json:"isDraft"`
	Version           int                 `json:"version"`
	CreatedAt         string              `json:"createdAt"`
	UpdatedAt         string              `json:"updatedAt"`
}

func RequestToResponse(r *entities.Request) RequestResponse {
	headers := HeaderItemsToDTO(r.Headers)
	if headers == nil {
		headers = []HeaderItemDTO{}
	}

	grpcMetadata := r.GRPCMetadata
	if grpcMetadata == nil {
		grpcMetadata = make(map[string][]string)
	}

	return RequestResponse{
		ID:                r.ID.String(),
		CollectionID:      r.CollectionID.String(),
		Name:              r.Name,
		Description:       r.Description,
		Protocol:          string(r.Protocol),
		Method:            string(r.Method),
		URL:               r.URL,
		Headers:           headers,
		Body:              r.Body,
		BodyType:          string(r.BodyType),
		AuthType:          string(r.AuthType),
		AuthData:          r.AuthData,
		PreScript:         r.PreScript,
		PostScript:        r.PostScript,
		GRPCService:       r.GRPCService,
		GRPCMethod:        r.GRPCMethod,
		GRPCProtoPath:     r.GRPCProtoPath,
		GRPCMetadata:      grpcMetadata,
		GraphQLQuery:      r.GraphQLQuery,
		GraphQLVariables:  r.GraphQLVariables,
		GraphQLSchemaPath: r.GraphQLSchemaPath,
		GraphQLOperation:  r.GraphQLOperation,
		SortOrder:         r.SortOrder,
		IsDraft:           r.IsDraft,
		Version:           r.Version,
		CreatedAt:         r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func RequestsToResponse(requests []*entities.Request) []RequestResponse {
	result := make([]RequestResponse, 0, len(requests))
	for _, r := range requests {
		result = append(result, RequestToResponse(r))
	}
	return result
}

type ScriptResultDTO struct {
	PreConsole  []string         `json:"preConsole"`
	PostConsole []string         `json:"postConsole"`
	Tests       []TestResultDTO  `json:"tests"`
	Errors      []ScriptErrorDTO `json:"errors"`
}

type TestResultDTO struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

type ScriptErrorDTO struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

// Returns nil if input is nil.
func ScriptResultToDTO(sr *entities.ScriptResult) *ScriptResultDTO {
	if sr == nil {
		return nil
	}
	preConsole := sr.PreConsole
	if preConsole == nil {
		preConsole = []string{}
	}
	postConsole := sr.PostConsole
	if postConsole == nil {
		postConsole = []string{}
	}
	out := &ScriptResultDTO{
		PreConsole:  preConsole,
		PostConsole: postConsole,
		Tests:       make([]TestResultDTO, 0, len(sr.Tests)),
		Errors:      make([]ScriptErrorDTO, 0, len(sr.Errors)),
	}
	for _, t := range sr.Tests {
		out.Tests = append(out.Tests, TestResultDTO{Name: t.Name, Passed: t.Passed, Error: t.Error})
	}
	for _, e := range sr.Errors {
		out.Errors = append(out.Errors, ScriptErrorDTO{Phase: e.Phase, Message: e.Message})
	}
	return out
}

type ExecuteResponseDTO struct {
	StatusCode        int                 `json:"statusCode"`
	StatusText        string              `json:"statusText"`
	URL               string              `json:"url"`
	Protocol          string              `json:"protocol"`
	Headers           map[string][]string `json:"headers"`
	Body              string              `json:"body"`
	Size              int64               `json:"size"`
	DurationMs        int64               `json:"durationMs"`
	IsBinary          bool                `json:"isBinary"`
	BinaryPath        string              `json:"binaryPath,omitempty"`
	SuggestedFilename string              `json:"suggestedFilename,omitempty"`
	ScriptResult      *ScriptResultDTO    `json:"scriptResult,omitempty"`
}

func ResponseToExecuteDTO(r *entities.Response) ExecuteResponseDTO {
	headers := r.Headers
	if headers == nil {
		headers = make(map[string][]string)
	}
	dto := ExecuteResponseDTO{
		StatusCode:        r.StatusCode,
		StatusText:        r.StatusText,
		URL:               r.URL,
		Protocol:          string(r.Protocol),
		Headers:           headers,
		Body:              r.Body,
		Size:              r.Size,
		DurationMs:        r.Duration.Milliseconds(),
		IsBinary:          r.IsBinary,
		BinaryPath:        r.BinaryPath,
		SuggestedFilename: r.SuggestedFilename,
	}
	dto.ScriptResult = ScriptResultToDTO(r.ScriptResult)
	return dto
}

type DeleteDraftRequest struct {
	ID string `json:"id"`
}

type PromoteDraftRequest struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	TargetCollectionID string `json:"targetCollectionId"`
	Version            int    `json:"version"`
}

type GenerateCurlRequest struct {
	RequestID   string `json:"requestId"`
	WorkspaceID string `json:"workspaceId"`
}

// Warnings name what the command could not carry, such as a missing OAuth 2.0 token.
type GenerateCurlResponse struct {
	Command      string           `json:"command"`
	Warnings     []string         `json:"warnings"`
	ScriptResult *ScriptResultDTO `json:"scriptResult,omitempty"`
}

type ParseCurlRequest struct {
	Text string `json:"text"`
}

// Warnings list what the parser dropped or could not honour.
type ParseCurlResponse struct {
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Headers  []HeaderItemDTO `json:"headers"`
	BodyType string          `json:"bodyType"`
	Body     string          `json:"body"`
	AuthType string          `json:"authType"`
	AuthData string          `json:"authData"`
	Warnings []string        `json:"warnings"`
}
