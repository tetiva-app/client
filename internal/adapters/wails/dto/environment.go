package dto

import "github.com/tetiva-app/client/internal/domain/entities"

type CreateEnvironmentRequest struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspaceId"`
}

type EditEnvironmentRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type DeleteEnvironmentRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type DuplicateEnvironmentRequest struct {
	SourceID    string `json:"sourceId"`
	NewName     string `json:"newName"`
	WorkspaceID string `json:"workspaceId"`
}

type SetActiveEnvironmentRequest struct {
	EnvironmentID string `json:"environmentId"`
}

type EnvironmentResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"isActive"`
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func EnvironmentToResponse(e *entities.Environment) EnvironmentResponse {
	return EnvironmentResponse{
		ID:        e.ID.String(),
		Name:      e.Name,
		IsActive:  e.IsActive,
		Version:   e.Version,
		CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: e.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func EnvironmentsToResponse(envs []*entities.Environment) []EnvironmentResponse {
	result := make([]EnvironmentResponse, 0, len(envs))
	for _, e := range envs {
		result = append(result, EnvironmentToResponse(e))
	}
	return result
}

type AddVariableRequest struct {
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Value         string `json:"value"`
	IsSecret      bool   `json:"isSecret"`
}

type EditVariableRequest struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
	Enabled  bool   `json:"enabled"`
	Version  int    `json:"version"`
}

type DeleteVariableRequest struct {
	ID string `json:"id"`
}

type VariableResponse struct {
	ID            string `json:"id"`
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Value         string `json:"value"`
	IsSecret      bool   `json:"isSecret"`
	Enabled       bool   `json:"enabled"`
	SortOrder     int    `json:"sortOrder"`
	Version       int    `json:"version"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func VariableToResponse(v *entities.Variable) VariableResponse {
	return VariableResponse{
		ID:            v.ID.String(),
		EnvironmentID: v.EnvironmentID.String(),
		Key:           v.Key,
		Value:         v.Value,
		IsSecret:      v.IsSecret,
		Enabled:       v.Enabled,
		SortOrder:     v.SortOrder,
		Version:       v.Version,
		CreatedAt:     v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     v.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func VariablesToResponse(vars []*entities.Variable) []VariableResponse {
	result := make([]VariableResponse, 0, len(vars))
	for _, v := range vars {
		result = append(result, VariableToResponse(v))
	}
	return result
}
