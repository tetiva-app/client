package entities

import "time"

type UpdatePhase string

const (
	UpdateIdle        UpdatePhase = "idle"
	UpdateChecking    UpdatePhase = "checking"
	UpdateUpToDate    UpdatePhase = "up_to_date"
	UpdateAvailable   UpdatePhase = "available"
	UpdateDownloading UpdatePhase = "downloading"
	UpdateReady       UpdatePhase = "ready"
	UpdateApplying    UpdatePhase = "applying"
	UpdateError       UpdatePhase = "error"
)

type InstallKind string

const (
	InstallInApp            InstallKind = "in_app"
	InstallAPT              InstallKind = "apt"
	InstallAPTNotConfigured InstallKind = "apt_not_configured"
	InstallUnsupported      InstallKind = "unsupported"
)

type Artifact struct {
	OS, Arch, Format, URL, SHA256 string
	Size                          int64
}

type Release struct {
	Version       string
	PublishedAt   time.Time
	InAppDisabled []string
	Artifacts     []Artifact
}

type UpdateState struct {
	Phase           UpdatePhase
	Version         string
	Current         string
	Received, Total int64
	Install         InstallKind
	Reason          string
}

type RestoreTab struct{ Type, ID string }

type RestoreTabs struct {
	WorkspaceID string
	Tabs        []RestoreTab
	ActiveTabID string
	SavedAt     time.Time
}
