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
	FormatTetiva     = "tetiva"
	FormatPostman    = "postman"
	FormatInsomnia   = "insomnia"
	FormatOpenAPI    = "openapi"
	FormatHAR        = "har"
	FormatHoppscotch = "hoppscotch"
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
	ReasonFileTooLarge     = "FILE_TOO_LARGE"
)

const MaxFileBytes = 48 << 20

func CheckFileSize(size int) error {
	if size <= MaxFileBytes {
		return nil
	}
	return &domain.ReasonError{Reason: ReasonFileTooLarge, Err: &domain.ValidationError{Fields: map[string]string{
		"content": fmt.Sprintf("larger than %d MiB", MaxFileBytes>>20),
	}}}
}

type TxRunner interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

// Importer turns one file format into collections; callers wrap Import in TxRunner.Run.
type Importer interface {
	Detect(data []byte) bool
	Preview(data []byte, fileName string) (*ImportPreview, error)
	Import(ctx context.Context, data []byte, opt ImportOpt) (*ImportResult, error)
}

type ImportOpt struct {
	WorkspaceID    uuid.UUID
	ParentID       *uuid.UUID
	UserID         string
	IncludeScripts bool
	FileName       string
}

// ImportPreview counts folders below the imported collection, not the collection itself.
type ImportPreview struct {
	Format           string
	Title            string
	Folders          int
	Requests         int
	Examples         int
	EnvironmentNames []string
	Hosts            []string
	Scripts          []ScriptPreview
	Warnings         []string
}

type ScriptPreview struct {
	Path  string
	Phase string
	Text  string
}

type ImportResult struct {
	CollectionID      uuid.UUID
	Folders           int
	Requests          int
	Examples          int
	EnvironmentNames  []string
	ActiveEnvironment string
	Warnings          []string
}

func Select(data []byte, importers ...Importer) (Importer, error) {
	for _, imp := range importers {
		if imp.Detect(data) {
			return imp, nil
		}
	}
	return nil, &domain.ReasonError{Reason: ReasonUnsupportedFile, Err: &domain.ValidationError{Fields: map[string]string{
		"content": "not a Tetiva, Postman v2.1, Insomnia or Hoppscotch collection, an OpenAPI / Swagger document or a HAR file",
	}}}
}

func Plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

const maxListed = 5

func Listing(items []string) string {
	if len(items) > maxListed {
		return fmt.Sprintf("%s and %d more", strings.Join(items[:maxListed], ", "), len(items)-maxListed)
	}
	return strings.Join(items, ", ")
}

func URLEncodedForms(requests []string) string {
	return Plural(len(requests), "multipart request has", "multipart requests have") +
		" no file field and will be sent as a URL-encoded form, not multipart: " + Listing(requests)
}

func FreeName(name string, taken map[string]bool) string {
	candidate := name
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s (%d)", name, n)
	}
	return candidate
}

var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

const maxHostExpansion = 4 << 10

func Hosts(urls []string, envs ...map[string]string) []string {
	if len(envs) == 0 {
		envs = []map[string]string{nil}
	}
	out := []string{}
	for _, vars := range envs {
		for _, raw := range urls {
			budget := maxHostExpansion
			if h := HostOf(ExpandVars(raw, vars, &budget)); h != "" && !onlyVariables(h) && !slices.Contains(out, h) {
				out = append(out, h)
			}
		}
	}
	slices.Sort(out)
	return out
}

func ExpandVars(s string, vars map[string]string, budget *int) string {
	return varPattern.ReplaceAllStringFunc(s, func(ref string) string {
		v, ok := vars[ref[2:len(ref)-2]]
		if !ok || len(v) > *budget {
			return ref
		}
		*budget -= len(v)
		return v
	})
}

func onlyVariables(host string) bool {
	rest := varPattern.ReplaceAllString(host, "")
	return rest != host && strings.Trim(rest, ":0123456789") == ""
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
