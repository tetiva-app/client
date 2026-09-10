package dto

import "github.com/tetiva-app/client/internal/domain/entities"

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

type EditWorkspaceRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type DeleteWorkspaceRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type SetActiveWorkspaceRequest struct {
	WorkspaceID string `json:"workspaceId"`
}

type WorkspaceResponse struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	IsActive          bool    `json:"isActive"`
	Version           int     `json:"version"`
	RemoteWorkspaceID *string `json:"remoteWorkspaceId"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

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

func WorkspacesToResponse(workspaces []*entities.Workspace) []WorkspaceResponse {
	result := make([]WorkspaceResponse, 0, len(workspaces))
	for _, w := range workspaces {
		result = append(result, WorkspaceToResponse(w))
	}
	return result
}

// Carries the workspace with the reason its cloud copy is missing: the local half
// succeeds even when the server refuses, and Result has room for one, not both.
type CreateRemoteWorkspaceResult struct {
	Workspace   WorkspaceResponse `json:"workspace"`
	SyncWarning string            `json:"syncWarning"`
}
