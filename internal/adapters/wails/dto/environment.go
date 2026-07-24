package dto

import "github.com/tetiva-app/client/internal/domain/entities"

// CreateEnvironmentRequest is the frontend request to create an environment.
type CreateEnvironmentRequest struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspaceId"`
}

// EditEnvironmentRequest is the frontend request to edit an environment.
type EditEnvironmentRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// DeleteEnvironmentRequest is the frontend request to delete an environment.
type DeleteEnvironmentRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// DuplicateEnvironmentRequest is the frontend request to duplicate an environment.
type DuplicateEnvironmentRequest struct {
	SourceID    string `json:"sourceId"`
	NewName     string `json:"newName"`
	WorkspaceID string `json:"workspaceId"`
}

// SetActiveEnvironmentRequest is the frontend request to set the active environment.
type SetActiveEnvironmentRequest struct {
	EnvironmentID string `json:"environmentId"`
}

// EnvironmentResponse is the frontend response representing an environment.
type EnvironmentResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"isActive"`
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// EnvironmentToResponse maps a domain Environment entity to EnvironmentResponse.
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

// EnvironmentsToResponse maps a slice of domain Environment entities to responses.
func EnvironmentsToResponse(envs []*entities.Environment) []EnvironmentResponse {
	result := make([]EnvironmentResponse, 0, len(envs))
	for _, e := range envs {
		result = append(result, EnvironmentToResponse(e))
	}
	return result
}

// AddVariableRequest is the frontend request to add a variable.
type AddVariableRequest struct {
	EnvironmentID string `json:"environmentId"`
	Key           string `json:"key"`
	Value         string `json:"value"`
	IsSecret      bool   `json:"isSecret"`
}

// EditVariableRequest is the frontend request to edit a variable.
type EditVariableRequest struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
	Enabled  bool   `json:"enabled"`
	Version  int    `json:"version"`
}

// DeleteVariableRequest is the frontend request to delete a variable.
type DeleteVariableRequest struct {
	ID string `json:"id"`
}

// VariableResponse is the frontend response representing a variable.
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

// VariableToResponse maps a domain Variable entity to VariableResponse.
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

// VariablesToResponse maps a slice of domain Variable entities to responses.
func VariablesToResponse(vars []*entities.Variable) []VariableResponse {
	result := make([]VariableResponse, 0, len(vars))
	for _, v := range vars {
		result = append(result, VariableToResponse(v))
	}
	return result
}
