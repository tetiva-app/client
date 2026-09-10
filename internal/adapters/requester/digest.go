package requester

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/icholy/digest"

	"github.com/tetiva-app/client/internal/domain"
)

// digestTransports caches one transport per workspace, origin and credential pair: the
// kept challenge saves a round trip, and separate credentials never share state.
type digestTransports struct {
	mu sync.Mutex
	m  map[string]*digestEntry
}

func (c *digestTransports) get(ws uuid.UUID, u *url.URL, username, password string, store CookieStore) *digestEntry {
	// The halves are hashed apart: "ab"+"c" and "a"+"bc" must not share a key.
	user, pass := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(password))
	key := strings.Join([]string{
		ws.String(),
		u.Scheme + "://" + u.Host,
		hex.EncodeToString(user[:]) + ":" + hex.EncodeToString(pass[:]),
	}, "|")

	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.m[key]; ok {
		return t
	}
	t := &digest.Transport{
		Username:      username,
		Password:      password,
		FindChallenge: findDigestChallenge,
		Transport:     challengeBodyBuffer{},
	}
	if store != nil && ws != uuid.Nil {
		// The transport outlives the request it was built for, so its jar reads
		// on a background context.
		t.Jar = newWorkspaceJar(context.Background(), store, ws)
	}
	entry := &digestEntry{transport: t, gate: make(chan struct{}, 1)}
	if c.m == nil {
		c.m = map[string]*digestEntry{}
	}
	c.m[key] = entry
	return entry
}

// digestEntry serialises the sends racing before a challenge is cached: icholy/digest
// starts every unchallenged request at nonce-count 00000001, which RFC 7616 servers reject.
type digestEntry struct {
	transport *digest.Transport

	// A channel, not a mutex: a send queued behind the priming exchange must
	// still answer its own deadline and the user's Cancel.
	gate   chan struct{}
	primed atomic.Bool
}

func (e *digestEntry) RoundTrip(req *http.Request) (*http.Response, error) {
	if e.primed.Load() {
		return e.transport.RoundTrip(req)
	}

	ctx := req.Context()
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-e.gate }()

	if e.primed.Load() {
		return e.transport.RoundTrip(req)
	}
	resp, err := e.transport.RoundTrip(req)
	// Only an answered challenge proves one is cached now: an endpoint that
	// needed no digest at all would otherwise open the gate for a racing storm.
	if err == nil && resp != nil && resp.Request != nil &&
		strings.HasPrefix(resp.Request.Header.Get("Authorization"), "Digest ") {
		e.primed.Store(true)
	}

	return resp, err
}

// challengeBodyBuffer keeps a 401 readable: digest.Transport drains and closes every 401
// before deciding, and one it cannot answer is handed back as the user's response.
type challengeBodyBuffer struct{}

func (challengeBodyBuffer) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || resp.Body == nil {
		return resp, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	resp.Body = &rewindOnClose{r: bytes.NewReader(body)}
	return resp, nil
}

// rewindOnClose replays its bytes after Close, so a drained and closed response
// can still be read once it reaches the requester.
type rewindOnClose struct{ r *bytes.Reader }

func (b *rewindOnClose) Read(p []byte) (int, error) { return b.r.Read(p) }

func (b *rewindOnClose) Close() error {
	_, err := b.r.Seek(0, io.SeekStart)
	return err
}

// validateDigestCreds mirrors the SigV4 check: without credentials the server
// answers a bare 401 that explains nothing.
func validateDigestCreds(username, password string) error {
	missing := map[string]string{}
	if strings.TrimSpace(username) == "" {
		missing["username"] = "required for Digest"
	}
	if strings.TrimSpace(password) == "" {
		missing["password"] = "required for Digest"
	}
	if len(missing) > 0 {
		return &domain.ValidationError{Fields: missing}
	}
	return nil
}

// findDigestChallenge picks the first answerable challenge: a supported algorithm (no
// -sess) with qop unset or auth. ErrNoChallenge hands the raw 401 back instead of failing.
func findDigestChallenge(h http.Header) (*digest.Challenge, error) {
	for _, header := range h.Values("WWW-Authenticate") {
		if !digest.IsDigest(header) {
			continue
		}
		chal, err := digest.ParseChallenge(header)
		if err != nil || !digest.CanDigest(chal) {
			continue
		}
		if len(chal.QOP) > 0 && !chal.SupportsQOP("auth") {
			continue
		}
		return chal, nil
	}
	return nil, digest.ErrNoChallenge
}

// unsupportedDigestChallenge turns a 401 with an unanswerable challenge into a readable
// error; an answerable one is a rejected credential and stays an ordinary response.
func unsupportedDigestChallenge(resp *http.Response) error {
	if resp.StatusCode != http.StatusUnauthorized {
		return nil
	}
	for _, header := range resp.Header.Values("WWW-Authenticate") {
		if !digest.IsDigest(header) {
			continue
		}
		chal, err := digest.ParseChallenge(header)
		if err != nil {
			continue
		}
		switch {
		case !digest.CanDigest(chal):
			return unsupportedDigestError(chal.Algorithm)
		case len(chal.QOP) > 0 && !chal.SupportsQOP("auth"):
			return unsupportedDigestError("qop=" + strings.Join(chal.QOP, ","))
		}
	}
	return nil
}

func unsupportedDigestError(what string) error {
	return &domain.ValidationError{Fields: map[string]string{
		"auth": "server requires " + what + ", which is not supported",
	}}
}
