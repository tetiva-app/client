package dto

import "github.com/tetiva-app/client/internal/domain/entities"

// CreateWorkspaceRequest is the frontend request to create a workspace.
type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

// EditWorkspaceRequest is the frontend request to edit a workspace.
type EditWorkspaceRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// DeleteWorkspaceRequest is the frontend request to delete a workspace.
type DeleteWorkspaceRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// SetActiveWorkspaceRequest is the frontend request to set the active workspace.
type SetActiveWorkspaceRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

// WorkspaceResponse is the frontend response representing a workspace.
type WorkspaceResponse struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	IsActive          bool    `json:"isActive"`
	Version           int     `json:"version"`
	RemoteWorkspaceID *string `json:"remoteWorkspaceId"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// WorkspaceToResponse maps a domain Workspace entity to a WorkspaceResponse DTO.
func WorkspaceToResponse(w *entities.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:                w.ID.String(),
		Name:              w.Name,
		IsActive:          w.IsActive,
		Version:           w.Version,
		RemoteWorkspaceID: w.RemoteWorkspaceID,
		CreatedAt:         w.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         w.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// WorkspacesToResponse maps a slice of domain Workspace entities to WorkspaceResponse DTOs.
func WorkspacesToResponse(workspaces []*entities.Workspace) []WorkspaceResponse {
	result := make([]WorkspaceResponse, 0, len(workspaces))
	for _, w := range workspaces {
		result = append(result, WorkspaceToResponse(w))
	}
	return result
}
