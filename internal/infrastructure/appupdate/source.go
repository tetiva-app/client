package appupdate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

type Source struct {
	URL, Version, OS string
	Client           *http.Client
}

// NewSource never follows redirects: the manifest comes from our host or not at all.
func NewSource(dataDir, version string) *Source {
	return &Source{
		URL:     ManifestURL(dataDir),
		Version: version,
		OS:      runtime.GOOS,
		Client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *Source) Fetch(ctx context.Context) ([]byte, error) {
	const funcName = "appupdate.Source.Fetch"
	u, err := url.Parse(s.URL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	q := u.Query()
	q.Set("v", s.Version)
	q.Set("os", s.OS)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	req.Header.Set("User-Agent", "Tetiva/"+s.Version)
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected status %s", funcName, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if len(raw) > MaxManifestBytes {
		return nil, fmt.Errorf("%s: manifest over %d bytes", funcName, MaxManifestBytes)
	}
	return raw, nil
}
