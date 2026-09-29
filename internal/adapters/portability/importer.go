// Package portability holds the Importer contract, format detection and the import preview.
package portability

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
)

const (
	FormatTetiva  = "tetiva"
	FormatPostman = "postman"
)

const (
	ReasonLinkNotFound     = "LINK_NOT_FOUND"
	ReasonPasswordRequired = "PASSWORD_REQUIRED"
	ReasonPasswordInvalid  = "PASSWORD_INVALID"
	ReasonRateLimited      = "RATE_LIMITED"
	ReasonPreviewExpired   = "PREVIEW_EXPIRED"
	ReasonUnsupportedFile  = "UNSUPPORTED_FILE"
	ReasonCollectionFile   = "COLLECTION_FILE"
	ReasonEnvironmentFile  = "ENVIRONMENT_FILE"
)

type TxRunner interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

// Importer turns one file format into collections; callers wrap Import in TxRunner.Run.
type Importer interface {
	Detect(data []byte) bool
	Preview(data []byte) (*ImportPreview, error)
	Import(ctx context.Context, data []byte, opt ImportOpt) (*ImportResult, error)
}

type ImportOpt struct {
	WorkspaceID    uuid.UUID
	ParentID       *uuid.UUID
	UserID         string
	IncludeScripts bool
}

// ImportPreview counts folders below the imported collection, not the collection itself.
type ImportPreview struct {
	Format          string
	Title           string
	Folders         int
	Requests        int
	Examples        int
	EnvironmentName string
	Hosts           []string
	Scripts         []ScriptPreview
	Warnings        []string
}

type ScriptPreview struct {
	Path  string
	Phase string
	Text  string
}

type ImportResult struct {
	CollectionID    uuid.UUID
	Folders         int
	Requests        int
	Examples        int
	EnvironmentName string
	Warnings        []string
}

func Select(data []byte, importers ...Importer) (Importer, error) {
	for _, imp := range importers {
		if imp.Detect(data) {
			return imp, nil
		}
	}
	return nil, &domain.ReasonError{Reason: ReasonUnsupportedFile, Err: &domain.ValidationError{Fields: map[string]string{
		"content": "not a Tetiva collection or a Postman Collection v2.1 file",
	}}}
}

func FreeName(name string, taken map[string]bool) string {
	candidate := name
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s (%d)", name, n)
	}
	return candidate
}

var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func Hosts(urls []string, vars map[string]string) []string {
	var out []string
	for _, raw := range urls {
		resolved := varPattern.ReplaceAllStringFunc(raw, func(ref string) string {
			if v, ok := vars[ref[2:len(ref)-2]]; ok {
				return v
			}
			return ref
		})
		if h := HostOf(resolved); h != "" && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	slices.Sort(out)
	return out
}

// HostOf is hand-rolled: url.Parse rejects a {{variable}} in the host.
func HostOf(raw string) string {
	s := strings.TrimSpace(raw)
	var host string
	scheme, rest, _ := strings.Cut(s, ":")
	switch strings.ToLower(scheme) {
	case "dns", "passthrough", "xds":
		// grpc-go dials the endpoint; the authority of a dns target only names the resolver.
		host = grpcEndpoint(rest)
	case "unix", "unix-abstract":
	default:
		host = authority(s)
	}
	if host == "" {
		return s
	}
	if !strings.Contains(host, "{{") {
		host = strings.ToLower(host)
	}
	return host
}

func authority(s string) string {
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// grpcEndpoint mirrors grpc-go's resolver.Target.Endpoint, minus the scheme.
func grpcEndpoint(rest string) string {
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	if !strings.HasPrefix(rest, "/") {
		return rest
	}
	if after, ok := strings.CutPrefix(rest, "//"); ok {
		_, rest, _ = strings.Cut(after, "/")
	} else {
		rest = rest[1:]
	}
	if unescaped, err := url.PathUnescape(rest); err == nil {
		rest = unescaped
	}
	return rest
}
