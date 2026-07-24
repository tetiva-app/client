package dto

// ImportCollectionRequest is the request payload for importing a Postman collection.
type ImportCollectionRequest struct {
	Content     string  `json:"content"`
	ParentID    *string `json:"parentId,omitempty"`
	WorkspaceID string  `json:"workspaceId"`
}

// ImportCollectionResponse reports what was imported.
type ImportCollectionResponse struct {
	FoldersCreated  int `json:"foldersCreated"`
	RequestsCreated int `json:"requestsCreated"`
}

// ExportCollectionRequest is the request payload for exporting a collection.
type ExportCollectionRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
}

// ImportEnvironmentRequest is the request payload for importing a Postman environment.
type ImportEnvironmentRequest struct {
	Content     string `json:"content"`
	WorkspaceID string `json:"workspaceId"`
}

// ImportEnvironmentResponse reports what was imported.
type ImportEnvironmentResponse struct {
	EnvironmentName  string `json:"environmentName"`
	VariablesCreated int    `json:"variablesCreated"`
}

// ExportEnvironmentRequest is the request payload for exporting an environment.
type ExportEnvironmentRequest struct {
	ID string `json:"id"`
}

// ExportResponse is the result of exporting to a file: either the saved path or a canceled flag.
type ExportResponse struct {
	Path     string `json:"path"`
	Canceled bool   `json:"canceled"`
}
