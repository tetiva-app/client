// Package publicapi reads published collections from the anonymous /pub endpoints.
package publicapi

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain"
)

const productionBaseURL = "https://api.tetiva.app"

const (
	requestTimeout   = 30 * time.Second
	maxMetaBytes     = 64 << 10
	maxSnapshotBytes = 8 << 20
)

var (
	ErrNotFound         = errors.New("publicapi: the collection is not published")
	ErrWrongPassword    = errors.New("publicapi: wrong password")
	ErrPasswordRequired = errors.New("publicapi: the collection needs a password")
	ErrRateLimited      = errors.New("publicapi: too many requests")
	ErrTooLarge         = errors.New("publicapi: the snapshot is larger than 8 MiB")
	// ErrUnreachable also covers a timeout and a gateway answering for a server that is down.
	ErrUnreachable = errors.New("publicapi: can't reach the server")

	errOverLimit = errors.New("response body over its size limit")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,40}-[a-z0-9]{8}$`)

func ValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}

type Meta struct {
	Slug             string
	Title            string
	Locale           string
	Visibility       string
	Revision         int
	UpdatedAt        time.Time
	PasswordRequired bool
}

type Client struct {
	baseURL string
	http    *http.Client
}

// New never follows redirects: a bearer token must not travel to another host.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Meta(ctx context.Context, slug string) (*Meta, error) {
	const funcName = "publicapi.Meta"

	req, err := c.newRequest(ctx, http.MethodGet, slug, "", nil)
	if err != nil {
		return nil, err
	}
	body, err := c.do(req, maxMetaBytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	var m struct {
		Slug             string `json:"slug"`
		Title            string `json:"title"`
		Locale           string `json:"locale"`
		Visibility       string `json:"visibility"`
		Revision         int    `json:"revision"`
		UpdatedAt        string `json:"updated_at"`
		PasswordRequired bool   `json:"password_required"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", funcName, err)
	}
	updated, _ := time.Parse(time.RFC3339, m.UpdatedAt)
	return &Meta{
		Slug: m.Slug, Title: m.Title, Locale: m.Locale, Visibility: m.Visibility,
		Revision: m.Revision, UpdatedAt: updated, PasswordRequired: m.PasswordRequired,
	}, nil
}

// Unlock trades the page password for a view token.
func (c *Client) Unlock(ctx context.Context, slug, password string) (string, error) {
	const funcName = "publicapi.Unlock"

	payload, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, slug, "/unlock", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	body, err := c.do(req, maxMetaBytes)
	if err != nil {
		// 400 is a password no page can have, 413 an oversized one.
		if code, ok := statusOf(err); ok && (code == http.StatusUnauthorized || code == http.StatusBadRequest ||
			code == http.StatusRequestEntityTooLarge) {
			return "", fmt.Errorf("%s: %w", funcName, ErrWrongPassword)
		}
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Token == "" {
		return "", fmt.Errorf("%s: the server returned no token", funcName)
	}
	return out.Token, nil
}

// Snapshot counts as an import (?src=app); bearer is a view or a one-time import token.
func (c *Client) Snapshot(ctx context.Context, slug, bearer string) ([]byte, error) {
	const funcName = "publicapi.Snapshot"

	req, err := c.newRequest(ctx, http.MethodGet, slug, "/snapshot.json?src=app", nil)
	if err != nil {
		return nil, err
	}
	// Set by hand, so the transport leaves gzip to us and the limit applies to its output.
	req.Header.Set("Accept-Encoding", "gzip")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	body, err := c.do(req, maxSnapshotBytes)
	if err != nil {
		if code, ok := statusOf(err); ok && code == http.StatusUnauthorized {
			return nil, fmt.Errorf("%s: %w", funcName, ErrPasswordRequired)
		}
		if errors.Is(err, errOverLimit) {
			return nil, fmt.Errorf("%s: %w", funcName, ErrTooLarge)
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return body, nil
}

func (c *Client) newRequest(ctx context.Context, method, slug, suffix string, body io.Reader) (*http.Request, error) {
	if !ValidSlug(slug) {
		return nil, &domain.ValidationError{Fields: map[string]string{"slug": "not a Tetiva share link"}}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/pub/"+url.PathEscape(slug)+suffix, body)
	if err != nil {
		return nil, fmt.Errorf("publicapi: %w", err)
	}
	req.Header.Set("User-Agent", constants.AppName+"/"+constants.AppVersion)
	return req, nil
}

type statusError struct{ code int }

func (e *statusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d", e.code)
}

func statusOf(err error) (int, bool) {
	var se *statusError
	if errors.As(err, &se) {
		return se.code, true
	}
	return 0, false
}

func (c *Client) do(req *http.Request, limit int64) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, &statusError{code: resp.StatusCode})
	default:
		return nil, &statusError{code: resp.StatusCode}
	}

	var src io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, readFailure("gzip", err)
		}
		defer func() { _ = zr.Close() }()
		src = zr
	}
	body, err := io.ReadAll(io.LimitReader(src, limit+1))
	if err != nil {
		return nil, readFailure("read body", err)
	}
	if int64(len(body)) > limit {
		return nil, errOverLimit
	}
	return body, nil
}

func readFailure(stage string, err error) error {
	var corrupt flate.CorruptInputError
	if errors.Is(err, gzip.ErrHeader) || errors.Is(err, gzip.ErrChecksum) || errors.As(err, &corrupt) {
		return fmt.Errorf("%s: %w", stage, err)
	}
	return fmt.Errorf("%w: %s: %w", ErrUnreachable, stage, err)
}
