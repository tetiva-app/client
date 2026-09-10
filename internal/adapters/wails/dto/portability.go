package dto

type ImportCollectionRequest struct {
	Content     string  `json:"content"`
	ParentID    *string `json:"parentId,omitempty"`
	WorkspaceID string  `json:"workspaceId"`
}

// Warnings name the items whose auth could not be imported as-is.
type ImportCollectionResponse struct {
	FoldersCreated  int      `json:"foldersCreated"`
	RequestsCreated int      `json:"requestsCreated"`
	Warnings        []string `json:"warnings"`
}

type ExportCollectionRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
}

type ImportEnvironmentRequest struct {
	Content     string `json:"content"`
	WorkspaceID string `json:"workspaceId"`
}

type ImportEnvironmentResponse struct {
	EnvironmentName  string `json:"environmentName"`
	VariablesCreated int    `json:"variablesCreated"`
}

type ExportEnvironmentRequest struct {
	ID string `json:"id"`
}

// Either the saved path or a canceled flag.
type ExportResponse struct {
	Path     string `json:"path"`
	Canceled bool   `json:"canceled"`
}
