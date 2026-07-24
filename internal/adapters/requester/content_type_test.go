package requester

import "testing"

func TestIsBinaryContentType(t *testing.T) {
	tests := []struct {
		name    string
		ct      string
		wantBin bool
	}{
		{"empty", "", false},
		{"text/plain", "text/plain", false},
		{"text/html", "text/html; charset=utf-8", false},
		{"application/json", "application/json", false},
		{"application/json charset", "application/json; charset=utf-8", false},
		{"application/xml", "application/xml", false},
		{"text/xml", "text/xml", false},
		{"application/javascript", "application/javascript", false},
		{"text/csv", "text/csv", false},
		{"application/yaml", "application/x-yaml", false},
		{"text/yaml", "text/yaml", false},

		{"image/png", "image/png", true},
		{"image/jpeg", "image/jpeg", true},
		{"image/gif", "image/gif", true},
		{"image/webp", "image/webp", true},
		{"image/svg+xml", "image/svg+xml", false}, // SVG is text
		{"audio/mpeg", "audio/mpeg", true},
		{"video/mp4", "video/mp4", true},
		{"application/pdf", "application/pdf", true},
		{"application/zip", "application/zip", true},
		{"application/gzip", "application/gzip", true},
		{"application/x-tar", "application/x-tar", true},
		{"application/octet-stream", "application/octet-stream", true},
		{"font/woff2", "font/woff2", true},
		{"application/wasm", "application/wasm", true},

		// OOXML — binary ZIP containers whose subtype contains the substring
		// "xml"; must not be classified as text.
		{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", true},
		{"xlsx with charset", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet; charset=binary", true},
		{"legacy xls", "application/vnd.ms-excel", true},

		// Structured syntax suffixes (RFC 6839) — text-based
		{"application/atom+xml", "application/atom+xml", false},
		{"application/rss+xml", "application/rss+xml", false},
		{"application/ld+json", "application/ld+json", false},
		{"application/vnd.api+json", "application/vnd.api+json", false},

		{"IMAGE/PNG uppercase", "IMAGE/PNG", true},
		{"application/graphql", "application/graphql", false},
		{"multipart/form-data", "multipart/form-data", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinaryContentType(tt.ct)
			if got != tt.wantBin {
				t.Errorf("isBinaryContentType(%q) = %v, want %v", tt.ct, got, tt.wantBin)
			}
		})
	}
}

func TestIsAttachment(t *testing.T) {
	tests := []struct {
		name string
		cd   string
		want bool
	}{
		{"empty", "", false},
		{"attachment", "attachment", true},
		{"attachment with filename", `attachment; filename="report.xlsx"`, true},
		{"attachment uppercase", "Attachment; filename=report.xlsx", true},
		{"attachment RFC5987", "attachment; filename*=utf-8''report.xlsx", true},
		{"inline", "inline", false},
		{"inline with filename", `inline; filename="preview.png"`, false},
		{"form-data (not a download)", `form-data; name="field"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAttachment(tt.cd)
			if got != tt.want {
				t.Errorf("isAttachment(%q) = %v, want %v", tt.cd, got, tt.want)
			}
		})
	}
}

func TestSuggestedFilename(t *testing.T) {
	tests := []struct {
		name               string
		contentDisposition string
		contentType        string
		want               string
	}{
		{
			"simple filename",
			`attachment; filename="photo.png"`,
			"image/png",
			"photo.png",
		},
		{
			"RFC 5987 encoded filename",
			"attachment; filename*=utf-8''%D0%A1%D0%BD%D0%B8%D0%BC%D0%BE%D0%BA%20%D1%8D%D0%BA%D1%80%D0%B0%D0%BD%D0%B0%202026-03-12.png",
			"application/octet-stream",
			"Снимок экрана 2026-03-12.png",
		},
		{
			"both filename and filename*",
			`attachment; filename="fallback.png"; filename*=utf-8''real%20name.png`,
			"image/png",
			"real name.png",
		},
		{
			"no Content-Disposition, fallback to CT extension",
			"",
			"image/png",
			"response.png",
		},
		{
			"no Content-Disposition, PDF",
			"",
			"application/pdf",
			"response.pdf",
		},
		{
			"no headers at all",
			"",
			"",
			"",
		},
		{
			"Content-Disposition without filename",
			"inline",
			"image/jpeg",
			"response.jfif",
		},
		{
			"path traversal in filename",
			`attachment; filename="../../etc/passwd"`,
			"application/octet-stream",
			"passwd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestedFilename(tt.contentDisposition, tt.contentType)
			if got != tt.want {
				t.Errorf("suggestedFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}
