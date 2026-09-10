package requester

import (
	"mime"
	"net/url"
	"path"
	"strings"
)

// textApplicationTypes are application/* media types that carry text payloads.
// Anything else under application/* defaults to binary.
var textApplicationTypes = map[string]bool{
	"application/json":                  true,
	"application/xml":                   true,
	"application/javascript":            true,
	"application/x-javascript":          true,
	"application/ecmascript":            true,
	"application/yaml":                  true,
	"application/x-yaml":                true,
	"application/graphql":               true,
	"application/csv":                   true,
	"application/x-www-form-urlencoded": true,
}

// isBinaryContentType classifies by the parsed media type, not substring match —
// the xlsx subtype contains "xml" yet is binary. Empty Content-Type means text.
func isBinaryContentType(ct string) bool {
	if ct == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType == "" {
		mediaType = strings.ToLower(strings.TrimSpace(ct))
		if i := strings.IndexByte(mediaType, ';'); i >= 0 {
			mediaType = strings.TrimSpace(mediaType[:i])
		}
	}

	if strings.HasPrefix(mediaType, "text/") {
		return false
	}

	// Structured syntax suffixes (RFC 6839): +xml, +json → text-based.
	// Covers image/svg+xml, application/atom+xml, application/ld+json, etc.
	if strings.HasSuffix(mediaType, "+xml") || strings.HasSuffix(mediaType, "+json") {
		return false
	}

	if textApplicationTypes[mediaType] {
		return false
	}

	for _, prefix := range []string{"image/", "audio/", "video/", "font/"} {
		if strings.HasPrefix(mediaType, prefix) {
			return true
		}
	}

	return strings.HasPrefix(mediaType, "application/")
}

// isAttachment reports whether Content-Disposition marks the response as a
// download; such responses are treated as binary regardless of Content-Type.
func isAttachment(contentDisposition string) bool {
	if contentDisposition == "" {
		return false
	}
	if disposition, _, err := mime.ParseMediaType(contentDisposition); err == nil {
		return strings.EqualFold(disposition, "attachment")
	}
	lower := strings.ToLower(strings.TrimSpace(contentDisposition))
	return lower == "attachment" || strings.HasPrefix(lower, "attachment;")
}

// Priority: Content-Disposition filename* (RFC 5987) > filename > Content-Type extension.
func suggestedFilename(contentDisposition, contentType string) string {
	if contentDisposition != "" {
		if name := parseContentDispositionFilename(contentDisposition); name != "" {
			return name
		}
	}

	if contentType != "" {
		mediaType, _, _ := mime.ParseMediaType(contentType)
		if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
			return "response" + exts[0]
		}
	}

	return ""
}

// Supports both filename="..." and filename*=utf-8”... (RFC 5987).
func parseContentDispositionFilename(header string) string {
	_, params, err := mime.ParseMediaType(header)
	if err == nil {
		if encoded, ok := params["filename*"]; ok {
			if name := decodeRFC5987(encoded); name != "" {
				return name
			}
		}
		if name, ok := params["filename"]; ok && name != "" {
			return path.Base(name)
		}
	}
	return ""
}

// decodeRFC5987 decodes a value like "utf-8”%D0%A1%D0%BD%D0%B8%D0%BC%D0%BE%D0%BA.png".
func decodeRFC5987(value string) string {
	// Format: charset'language'encoded_value
	parts := strings.SplitN(value, "'", 3)
	if len(parts) != 3 {
		return ""
	}
	decoded, err := url.PathUnescape(parts[2])
	if err != nil {
		return ""
	}
	return path.Base(decoded)
}
