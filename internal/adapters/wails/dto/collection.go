package dto

import "github.com/tetiva-app/client/internal/domain/entities"

type CreateCollectionRequest struct {
	Name        string  `json:"name"`
	ParentID    *string `json:"parentId,omitempty"`
	Description string  `json:"description"`
	AuthType    string  `json:"authType"`
	AuthData    string  `json:"authData"`
	WorkspaceID string  `json:"workspaceId"`
}

type EditCollectionRequest struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	PreScript    string          `json:"preScript"`
	PostScript   string          `json:"postScript"`
	Description  string          `json:"description"`
	AuthType     string          `json:"authType"`
	AuthData     string          `json:"authData"`
	GRPCMetadata []HeaderItemDTO `json:"grpcMetadata"`
	Version      int             `json:"version"`
}

type DeleteCollectionRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type ReorderCollectionRequest struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sortOrder"`
}

type MoveCollectionRequest struct {
	ID             string  `json:"id"`
	TargetParentID *string `json:"targetParentId"`
	Version        int     `json:"version"`
}

type CollectionResponse struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspaceId"`
	ParentID     *string         `json:"parentId,omitempty"`
	Name         string          `json:"name"`
	PreScript    string          `json:"preScript"`
	PostScript   string          `json:"postScript"`
	Description  string          `json:"description"`
	AuthType     string          `json:"authType"`
	AuthData     string          `json:"authData"`
	GRPCMetadata []HeaderItemDTO `json:"grpcMetadata"`
	SortOrder    int             `json:"sortOrder"`
	Version      int             `json:"version"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

func CollectionToResponse(c *entities.Collection) CollectionResponse {
	resp := CollectionResponse{
		ID:           c.ID.String(),
		WorkspaceID:  c.WorkspaceID.String(),
		Name:         c.Name,
		PreScript:    c.PreScript,
		PostScript:   c.PostScript,
		Description:  c.Description,
		AuthType:     string(c.AuthType),
		AuthData:     c.AuthData,
		GRPCMetadata: HeaderItemsToDTO(c.GRPCMetadata),
		SortOrder:    c.SortOrder,
		Version:      c.Version,
		CreatedAt:    c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if c.ParentID != nil {
		s := c.ParentID.String()
		resp.ParentID = &s
	}
	return resp
}

func CollectionsToResponse(collections []*entities.Collection) []CollectionResponse {
	result := make([]CollectionResponse, 0, len(collections))
	for _, c := range collections {
		result = append(result, CollectionToResponse(c))
	}
	return result
}
