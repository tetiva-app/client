package requester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const maxResponseBody = 50 * 1024 * 1024 // 50 MB

// HTTPRequester sends HTTP requests with a per-workspace cookie jar: each
// Execute clones the jar-less base client with a workspaceJar for the request.
type HTTPRequester struct {
	base    *http.Client
	store   CookieStore
	digests *digestTransports
}

// store may be nil only in tests.
func NewHTTPRequester(store CookieStore) *HTTPRequester {
	return &HTTPRequester{
		base: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("too many redirects (max 10)")
				}
				return nil
			},
		},
		store:   store,
		digests: &digestTransports{},
	}
}

// stopAtRedirect keeps a signed or challenge-bound request on the URL it was
// built for; the 3xx is returned to the user like any other response.
func stopAtRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// Execute sends an HTTP request, attaching/persisting cookies per req.WorkspaceID.
func (r *HTTPRequester) Execute(ctx context.Context, req request.HTTPExecuteRequest) (*entities.Response, error) {
	var bodyReader io.Reader
	if req.BodyReader != nil {
		bodyReader = req.BodyReader
	} else if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	authType := entities.AuthTypeNone
	var authFields auth.Fields
	if req.Auth != nil {
		authType = req.Auth.Type
		authFields = auth.Fields(req.Auth.Fields)
	}

	payloadHash := emptyPayloadHash
	if needsSpooledBody(authType) && bodyReader != nil {
		spooled, spoolErr := spoolBody(bodyReader, maxSpooledBody)
		if spoolErr != nil {
			return nil, spoolErr
		}
		payloadHash = hashPayload(spooled)
		bodyReader = spooled
	}

	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range req.Headers {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	clientCopy := *r.base
	var jar *workspaceJar
	if r.store != nil && req.WorkspaceID != uuid.Nil {
		jar = newWorkspaceJar(ctx, r.store, req.WorkspaceID)
		clientCopy.Jar = jar
	}

	switch authType {
	case entities.AuthTypeDigest:
		username, password := authFields.Str("username"), authFields.Str("password")
		if credErr := validateDigestCreds(username, password); credErr != nil {
			return nil, credErr
		}
		// The digest transport adds jar cookies itself, so the client must not
		// also hold the jar — the 401 with the challenge never reaches it.
		clientCopy.Jar = nil
		clientCopy.CheckRedirect = stopAtRedirect
		clientCopy.Transport = r.digests.get(req.WorkspaceID, httpReq.URL, username, password, r.store)
	case entities.AuthTypeAWSSigV4:
		clientCopy.CheckRedirect = stopAtRedirect
		if signErr := signSigV4(httpReq, payloadHash, sigv4CredsFromFields(authFields), time.Now()); signErr != nil {
			return nil, signErr
		}
	}

	start := time.Now()
	resp, err := clientCopy.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		return nil, classifyError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if authType == entities.AuthTypeDigest {
		if chalErr := unsupportedDigestChallenge(resp); chalErr != nil {
			return nil, chalErr
		}
		// Only the digest transport saw the challenge response, so the cookies
		// of the final one are persisted here.
		if jar != nil {
			jar.SetCookies(httpReq.URL, resp.Cookies())
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	headers := make(map[string][]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[k] = v
	}

	ct := resp.Header.Get("Content-Type")
	cd := resp.Header.Get("Content-Disposition")
	if isAttachment(cd) || isBinaryContentType(ct) {
		tmpFile, tmpErr := os.CreateTemp("", "gopher-response-*")
		if tmpErr != nil {
			return nil, fmt.Errorf("HTTPRequester.Execute: failed to create temp file: %w", tmpErr)
		}
		if _, writeErr := tmpFile.Write(body); writeErr != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("HTTPRequester.Execute: failed to write temp file: %w", writeErr)
		}
		if closeErr := tmpFile.Close(); closeErr != nil {
			_ = os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("HTTPRequester.Execute: failed to close temp file: %w", closeErr)
		}

		return &entities.Response{
			StatusCode:        resp.StatusCode,
			StatusText:        resp.Status,
			URL:               req.URL,
			Headers:           headers,
			Body:              "",
			Size:              int64(len(body)),
			Duration:          duration,
			Protocol:          entities.ProtocolHTTP,
			IsBinary:          true,
			BinaryPath:        tmpFile.Name(),
			SuggestedFilename: suggestedFilename(cd, ct),
		}, nil
	}

	return &entities.Response{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		URL:        req.URL,
		Headers:    headers,
		Body:       string(body),
		Size:       int64(len(body)),
		Duration:   duration,
		Protocol:   entities.ProtocolHTTP,
	}, nil
}

// needsSpooledBody reports whether the scheme needs the body in memory: Digest
// may replay it, SigV4 hashes it before signing.
func needsSpooledBody(t entities.AuthType) bool {
	return t == entities.AuthTypeDigest || t == entities.AuthTypeAWSSigV4
}

func classifyError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return errors.New("request timed out after 30s")
		}

		var opErr *net.OpError
		if errors.As(urlErr.Err, &opErr) {
			if opErr.Op == "dial" {
				var dnsErr *net.DNSError
				if errors.As(opErr.Err, &dnsErr) {
					return fmt.Errorf("DNS resolution failed for %q", dnsErr.Name)
				}
				return fmt.Errorf("connection refused: %s", opErr.Addr)
			}
		}
	}

	return fmt.Errorf("request failed: %w", err)
}

// Used by curl-builder and the response-viewer "Sent" section.
func (r *HTTPRequester) CookiesFor(ctx context.Context, ws uuid.UUID, rawURL string) []*http.Cookie {
	if r.store == nil || ws == uuid.Nil {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	return r.store.GetCookiesFor(ctx, ws, u)
}
