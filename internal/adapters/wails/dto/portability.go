package dto

import (
	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/hoppscotch"
	"github.com/tetiva-app/client/internal/adapters/portability/postman"
)

type ImportCollectionRequest struct {
	Content     string  `json:"content"`
	ParentID    *string `json:"parentId,omitempty"`
	WorkspaceID string  `json:"workspaceId"`
}

// Warnings name what could not be imported as-is.
type ImportCollectionResponse struct {
	FoldersCreated  int             `json:"foldersCreated"`
	RequestsCreated int             `json:"requestsCreated"`
	Warnings        []ImportWarning `json:"warnings"`
}

type ImportWarningSubject struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// Text is the English rendering; the UI words the note from Code and Params.
type ImportWarning struct {
	Code    string               `json:"code"`
	Subject ImportWarningSubject `json:"subject"`
	Params  map[string]string    `json:"params"`
	Text    string               `json:"text"`
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
	EnvironmentID    string          `json:"environmentId"`
	EnvironmentNames []string        `json:"environmentNames"`
	VariablesCreated int             `json:"variablesCreated"`
	Warnings         []ImportWarning `json:"warnings"`
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
	Content  string `json:"content"`
	FileName string `json:"fileName,omitempty"`
}

type ScriptPreview struct {
	Path  string `json:"path"`
	Phase string `json:"phase"`
	Text  string `json:"text"`
}

type ImportPreview struct {
	Format           string          `json:"format"`
	Title            string          `json:"title"`
	Folders          int             `json:"folders"`
	Requests         int             `json:"requests"`
	Examples         int             `json:"examples"`
	EnvironmentNames []string        `json:"environmentNames"`
	Hosts            []string        `json:"hosts"`
	Scripts          []ScriptPreview `json:"scripts"`
	Warnings         []ImportWarning `json:"warnings"`
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
	FileName       string  `json:"fileName,omitempty"`
}

type ImportConfirmResult struct {
	CollectionID      string          `json:"collectionId"`
	Folders           int             `json:"folders"`
	Requests          int             `json:"requests"`
	Examples          int             `json:"examples"`
	EnvironmentNames  []string        `json:"environmentNames"`
	ActiveEnvironment string          `json:"activeEnvironment"`
	Warnings          []ImportWarning `json:"warnings"`
}

func ImportWarningsFrom(ws []portability.Warning) []ImportWarning {
	out := make([]ImportWarning, 0, len(ws))
	for _, w := range ws {
		params := w.Params
		if params == nil {
			params = map[string]string{}
		}
		out = append(out, ImportWarning{
			Code: string(w.Code), Subject: ImportWarningSubject{Kind: w.Subject.Kind, Name: w.Subject.Name}, Params: params, Text: w.Text,
		})
	}
	return out
}

func ImportPreviewFrom(p *portability.ImportPreview) ImportPreview {
	scripts := make([]ScriptPreview, 0, len(p.Scripts))
	for _, s := range p.Scripts {
		scripts = append(scripts, ScriptPreview{Path: s.Path, Phase: s.Phase, Text: s.Text})
	}
	return ImportPreview{
		Format: p.Format, Title: p.Title, Folders: p.Folders, Requests: p.Requests, Examples: p.Examples,
		EnvironmentNames: nonNilStrings(p.EnvironmentNames), Hosts: nonNilStrings(p.Hosts), Scripts: scripts, Warnings: ImportWarningsFrom(p.Warnings),
	}
}

func ImportConfirmResultFrom(r *portability.ImportResult) ImportConfirmResult {
	return ImportConfirmResult{
		CollectionID: r.CollectionID.String(), Folders: r.Folders, Requests: r.Requests, Examples: r.Examples,
		EnvironmentNames: nonNilStrings(r.EnvironmentNames), ActiveEnvironment: r.ActiveEnvironment,
		Warnings: ImportWarningsFrom(r.Warnings),
	}
}

func ImportEnvironmentResponseFrom(r *postman.ImportEnvResult) ImportEnvironmentResponse {
	return ImportEnvironmentResponse{
		EnvironmentID: r.EnvironmentID.String(), EnvironmentNames: []string{r.EnvironmentName},
		VariablesCreated: r.VariablesCreated, Warnings: ImportWarningsFrom(r.Warnings),
	}
}

func HoppscotchEnvironmentResponseFrom(r *hoppscotch.EnvironmentResult) ImportEnvironmentResponse {
	return ImportEnvironmentResponse{
		EnvironmentID: r.ID.String(), EnvironmentNames: nonNilStrings(r.Names),
		VariablesCreated: r.Variables, Warnings: ImportWarningsFrom(r.Warnings),
	}
}
