package appupdate

import "time"

// manifestDTO keeps the top-level version and url that clients up to 1.2.0 read.
type manifestDTO struct {
	Version    string   `json:"version"`
	URL        string   `json:"url"`
	Payload    string   `json:"payload"`
	Signatures []string `json:"signatures"`
}

type payloadDTO struct {
	Version       string        `json:"version"`
	PublishedAt   time.Time     `json:"publishedAt"`
	InAppDisabled []string      `json:"inAppDisabled"`
	Artifacts     []artifactDTO `json:"artifacts"`
}

type artifactDTO struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Format string `json:"format"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type restoreDTO struct {
	WorkspaceID string          `json:"workspaceId"`
	Tabs        []restoreTabDTO `json:"tabs"`
	ActiveTabID string          `json:"activeTabId"`
	SavedAt     time.Time       `json:"savedAt"`
}

type restoreTabDTO struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
