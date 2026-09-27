package dto

import "github.com/tetiva-app/client/internal/adapters/portability"

type ImportCollectionRequest struct {
	Content     string  `json:"content"`
	ParentID    *string `json:"parentId,omitempty"`
	WorkspaceID string  `json:"workspaceId"`
}

// Warnings name what could not be imported as-is.
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
	Path     string   `json:"path"`
	Canceled bool     `json:"canceled"`
	Warnings []string `json:"warnings"`
}

type LinkMetaRequest struct {
	Slug string `json:"slug"`
}

type LinkMeta struct {
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	PasswordRequired bool   `json:"passwordRequired"`
	Revision         int    `json:"revision"`
	UpdatedAt        string `json:"updatedAt"`
}

type LinkUnlockRequest struct {
	Slug     string `json:"slug"`
	Password string `json:"password"`
}

type LinkUnlockResult struct {
	Token string `json:"token"`
}

type LinkFetchRequest struct {
	Slug  string `json:"slug"`
	Token string `json:"token"`
}

type ImportPreviewRequest struct {
	Content string `json:"content"`
}

type ScriptPreview struct {
	Path  string `json:"path"`
	Phase string `json:"phase"`
	Text  string `json:"text"`
}

type ImportPreview struct {
	Format          string          `json:"format"`
	Title           string          `json:"title"`
	Folders         int             `json:"folders"`
	Requests        int             `json:"requests"`
	Examples        int             `json:"examples"`
	EnvironmentName string          `json:"environmentName"`
	Hosts           []string        `json:"hosts"`
	Scripts         []ScriptPreview `json:"scripts"`
	Warnings        []string        `json:"warnings"`
}

type ImportPreviewResult struct {
	PreviewID string        `json:"previewId"`
	Preview   ImportPreview `json:"preview"`
}

type ImportConfirmRequest struct {
	PreviewID      string  `json:"previewId,omitempty"`
	Content        string  `json:"content,omitempty"`
	IncludeScripts bool    `json:"includeScripts"`
	WorkspaceID    string  `json:"workspaceId"`
	ParentID       *string `json:"parentId,omitempty"`
}

type ImportConfirmResult struct {
	CollectionID string   `json:"collectionId"`
	Folders      int      `json:"folders"`
	Requests     int      `json:"requests"`
	Examples     int      `json:"examples"`
	Warnings     []string `json:"warnings"`
}

func ImportPreviewFrom(p *portability.ImportPreview) ImportPreview {
	scripts := make([]ScriptPreview, 0, len(p.Scripts))
	for _, s := range p.Scripts {
		scripts = append(scripts, ScriptPreview{Path: s.Path, Phase: s.Phase, Text: s.Text})
	}
	return ImportPreview{
		Format: p.Format, Title: p.Title, Folders: p.Folders, Requests: p.Requests, Examples: p.Examples,
		EnvironmentName: p.EnvironmentName, Hosts: nonNilStrings(p.Hosts), Scripts: scripts, Warnings: nonNilStrings(p.Warnings),
	}
}

func ImportConfirmResultFrom(r *portability.ImportResult) ImportConfirmResult {
	return ImportConfirmResult{
		CollectionID: r.CollectionID.String(), Folders: r.Folders, Requests: r.Requests, Examples: r.Examples,
		Warnings: nonNilStrings(r.Warnings),
	}
}
