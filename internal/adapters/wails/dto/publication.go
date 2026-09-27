package dto

import "github.com/tetiva-app/client/internal/domain/usecase/publication"

type PublicationStatusRequest struct {
	CollectionID string `json:"collectionId"`
}

// PublicationStatus.ReasonUnavailable is "" | not_logged_in | no_capability | not_root | offline;
// HasChanges is yes | no | unknown; Settings is nil unless the caller can manage the publication.
// UnpublishError is set once the server has refused a pending unpublish several times.
type PublicationStatus struct {
	Available         bool                `json:"available"`
	ReasonUnavailable string              `json:"reasonUnavailable"`
	Published         bool                `json:"published"`
	CanManage         bool                `json:"canManage"`
	Stale             bool                `json:"stale"`
	Slug              string              `json:"slug"`
	PublicURL         string              `json:"publicUrl"`
	Visibility        string              `json:"visibility"`
	Revision          int                 `json:"revision"`
	UpdatedAt         string              `json:"updatedAt"`
	Blocked           bool                `json:"blocked"`
	BlockedReason     string              `json:"blockedReason"`
	Badge             bool                `json:"badge"`
	HasChanges        string              `json:"hasChanges"`
	Settings          *PublishSettings    `json:"settings"`
	Counters          PublicationCounters `json:"counters"`
	PendingUnpublish  bool                `json:"pendingUnpublish"`
	UnpublishError    string              `json:"unpublishError"`
}

type PublicationListRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Remote      bool   `json:"remote"`
}

type PublicationListItem struct {
	CollectionID string            `json:"collectionId"`
	Name         string            `json:"name"`
	Status       PublicationStatus `json:"status"`
}

type PublicationList struct {
	Reason string                `json:"reason"`
	Items  []PublicationListItem `json:"items"`
}

type PublishPlanRequest struct {
	CollectionID string `json:"collectionId"`
}

// PublishPlan says which paid visibilities the plan behind the collection includes.
type PublishPlan struct {
	Unlisted bool `json:"unlisted"`
	Password bool `json:"password"`
}

type PublishSettings struct {
	EnvironmentID      string   `json:"environmentId"`
	EnvironmentName    string   `json:"environmentName"`
	EnvironmentMissing bool     `json:"environmentMissing"`
	IncludeScripts     bool     `json:"includeScripts"`
	PublishAsIs        []string `json:"publishAsIs"`
}

type PublicationCounters struct {
	Views     int64 `json:"views"`
	Imports   int64 `json:"imports"`
	Downloads int64 `json:"downloads"`
}

type PublishPreviewRequest struct {
	CollectionID   string   `json:"collectionId"`
	WorkspaceID    string   `json:"workspaceId"`
	EnvironmentID  string   `json:"environmentId"`
	IncludeScripts bool     `json:"includeScripts"`
	PublishAsIs    []string `json:"publishAsIs"`
}

type PublishPreview struct {
	Folders          int             `json:"folders"`
	Requests         int             `json:"requests"`
	Examples         int             `json:"examples"`
	PublishedVars    []string        `json:"publishedVars"`
	HiddenVars       []HiddenVar     `json:"hiddenVars"`
	Redactions       []Redaction     `json:"redactions"`
	Warnings         []ScanWarning   `json:"warnings"`
	Errors           []BlockingError `json:"errors"`
	IgnoredOverrides []string        `json:"ignoredOverrides"`
	SizeBytes        int             `json:"sizeBytes"`
	SizeLimitBytes   int             `json:"sizeLimitBytes"`
	GzipBytes        int             `json:"gzipBytes"`
	GzipLimitBytes   int             `json:"gzipLimitBytes"`
	LargestExamples  []SizedExample  `json:"largestExamples"`
	PreviewHash      string          `json:"previewHash"`
}

// SizedExample is a response example by body size, listed when the snapshot is over its limit.
type SizedExample struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

type HiddenVar struct {
	VariableID  string `json:"variableId"`
	Key         string `json:"key"`
	Reason      string `json:"reason"`
	Selector    string `json:"selector"`
	Overridable bool   `json:"overridable"`
	Overridden  bool   `json:"overridden"`
}

type Redaction struct {
	Selector    string `json:"selector"`
	Path        string `json:"path"`
	Category    string `json:"category"`
	Reason      string `json:"reason"`
	Overridable bool   `json:"overridable"`
	Overridden  bool   `json:"overridden"`
}

type ScanWarning struct {
	Selector   string `json:"selector"`
	Path       string `json:"path"`
	Rule       string `json:"rule"`
	Excerpt    string `json:"excerpt"`
	Overridden bool   `json:"overridden"`
}

type BlockingError struct {
	Path    string            `json:"path"`
	Code    string            `json:"code"`
	Params  map[string]string `json:"params"`
	Message string            `json:"message"`
}

// PublishRequest.Locale is ru | en; Password is sent only with the password visibility.
type PublishRequest struct {
	CollectionID         string   `json:"collectionId"`
	WorkspaceID          string   `json:"workspaceId"`
	Visibility           string   `json:"visibility"`
	Password             *string  `json:"password"`
	EnvironmentID        string   `json:"environmentId"`
	IncludeScripts       bool     `json:"includeScripts"`
	PublishAsIs          []string `json:"publishAsIs"`
	Locale               string   `json:"locale"`
	ConfirmMakePublic    bool     `json:"confirmMakePublic"`
	AcknowledgedWarnings bool     `json:"acknowledgedWarnings"`
	PreviewHash          string   `json:"previewHash"`
}

type UnpublishRequest struct {
	CollectionID string `json:"collectionId"`
}

type MarkSecretRequest struct {
	EnvironmentID string `json:"environmentId"`
	VariableID    string `json:"variableId"`
}

// PublishPreviewFromReport leaves the sizes and the hash to the caller.
func PublishPreviewFromReport(r publication.Report) PublishPreview {
	out := PublishPreview{
		Folders:          r.Folders,
		Requests:         r.Requests,
		Examples:         r.Examples,
		PublishedVars:    nonNilStrings(r.PublishedVars),
		HiddenVars:       make([]HiddenVar, 0, len(r.HiddenVars)),
		Redactions:       make([]Redaction, 0, len(r.Redactions)),
		Warnings:         make([]ScanWarning, 0, len(r.Warnings)),
		Errors:           make([]BlockingError, 0, len(r.Errors)),
		IgnoredOverrides: nonNilStrings(r.IgnoredOverrides),
		LargestExamples:  []SizedExample{},
	}
	for _, v := range r.HiddenVars {
		out.HiddenVars = append(out.HiddenVars, HiddenVar{
			VariableID: v.VariableID.String(), Key: v.Key, Reason: v.Reason, Selector: v.Selector,
			Overridable: v.Overridable, Overridden: v.Overridden,
		})
	}
	for _, rd := range r.Redactions {
		out.Redactions = append(out.Redactions, Redaction{
			Selector: rd.Selector, Path: rd.Path, Category: rd.Category, Reason: rd.Reason,
			Overridable: rd.Overridable, Overridden: rd.Overridden,
		})
	}
	for _, w := range r.Warnings {
		out.Warnings = append(out.Warnings, ScanWarning{
			Selector: w.Selector, Path: w.Path, Rule: w.Rule, Excerpt: w.Excerpt, Overridden: w.Overridden,
		})
	}
	for _, e := range r.Errors {
		out.Errors = append(out.Errors, BlockingError{Path: e.Path, Code: e.Code, Params: e.Params, Message: e.Message})
	}
	return out
}
