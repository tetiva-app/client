package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startHTTPServer binds the MCP server on a free loopback port and returns its base URL.
func startHTTPServer(t *testing.T, srv *Server) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	srv.addr = addr
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	base := "http://" + addr
	waitForListener(t, base)
	return base
}

func waitForListener(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", strings.TrimPrefix(base, "http://"), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("MCP server did not start listening on %s", base)
}

// sseSessionClient drives a real MCP handshake: open /sse, take the endpoint
// event, POST to it, and read the answer back off the same stream.
type sseSessionClient struct {
	t        *testing.T
	base     string
	header   string
	body     io.ReadCloser
	reader   *bufio.Reader
	endpoint string
}

// openSSE connects to /sse. query and header are the two token transports.
func openSSE(t *testing.T, base, query, header string) (*sseSessionClient, int) {
	t.Helper()

	url := base + "/sse"
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	if header != "" {
		req.Header.Set("Authorization", header)
	}

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, resp.StatusCode
	}

	c := &sseSessionClient{t: t, base: base, header: header, body: resp.Body, reader: bufio.NewReader(resp.Body)}
	t.Cleanup(func() { _ = c.body.Close() })
	c.endpoint = c.readEvent("endpoint")
	return c, resp.StatusCode
}

// readEvent returns the data payload of the next event with the given name.
func (c *sseSessionClient) readEvent(name string) string {
	c.t.Helper()

	done := make(chan string, 1)
	go func() {
		var current string
		for {
			line, err := c.reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				current = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: ") && current == name:
				done <- strings.TrimPrefix(line, "data: ")
				return
			}
		}
	}()

	select {
	case data := <-done:
		return data
	case <-time.After(5 * time.Second):
		c.t.Fatalf("timed out waiting for SSE event %q", name)
		return ""
	}
}

// call posts a JSON-RPC request to the session endpoint and returns the HTTP
// status; the JSON-RPC answer arrives over the SSE stream.
func (c *sseSessionClient) call(method string, params any) int {
	c.t.Helper()

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	require.NoError(c.t, err)

	req, err := http.NewRequest(http.MethodPost, c.base+c.endpoint, bytes.NewReader(payload))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	if c.header != "" {
		req.Header.Set("Authorization", c.header)
	}

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(c.t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// callTools runs the full cycle and returns the decoded tools/call result.
func (c *sseSessionClient) callTools(name string, args map[string]any) map[string]any {
	c.t.Helper()

	status := c.call("tools/call", map[string]any{"name": name, "arguments": args})
	require.Equal(c.t, http.StatusAccepted, status, "POST /message rejected")

	var envelope map[string]any
	require.NoError(c.t, json.Unmarshal([]byte(c.readEvent("message")), &envelope))
	require.Nil(c.t, envelope["error"], "tools/call returned a JSON-RPC error: %v", envelope["error"])

	result, ok := envelope["result"].(map[string]any)
	require.True(c.t, ok, "no result in %v", envelope)
	return result
}

func requireToolText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]any)
	require.True(t, ok && len(content) > 0, "no content in %v", result)
	item, ok := content[0].(map[string]any)
	require.True(t, ok)
	return fmt.Sprint(item["text"])
}

func TestAuth_NoToken_Rejects(t *testing.T) {
	srv := setupTestServer(t)
	srv.auth.Set("s3cret", true)
	base := startHTTPServer(t, srv)

	_, status := openSSE(t, base, "", "")
	assert.Equal(t, http.StatusUnauthorized, status)

	// /message is guarded independently — a leaked session id must not be enough.
	resp, err := http.Post(base+"/message?sessionId=whatever", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, string(body), "Settings")
}

func TestAuth_WrongToken_Rejects(t *testing.T) {
	srv := setupTestServer(t)
	srv.auth.Set("s3cret", true)
	base := startHTTPServer(t, srv)

	_, status := openSSE(t, base, "token=nope", "")
	assert.Equal(t, http.StatusUnauthorized, status)

	_, status = openSSE(t, base, "", "Bearer nope")
	assert.Equal(t, http.StatusUnauthorized, status)

	// Different length must be rejected, not panic in the constant-time compare.
	_, status = openSSE(t, base, "", "Bearer "+strings.Repeat("x", 512))
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestAuth_BearerHeader_FullCycle(t *testing.T) {
	srv := setupTestServer(t)
	srv.auth.Set("s3cret", true)
	base := startHTTPServer(t, srv)

	c, status := openSSE(t, base, "", "bearer s3cret")
	require.Equal(t, http.StatusOK, status)

	text := requireToolText(t, c.callTools("list_workspaces", map[string]any{}))
	assert.Contains(t, text, "workspaces")
}

func TestAuth_QueryParam_FullCycle(t *testing.T) {
	srv := setupTestServer(t)
	srv.auth.Set("s3cret", true)
	base := startHTTPServer(t, srv)

	c, status := openSSE(t, base, "token=s3cret", "")
	require.Equal(t, http.StatusOK, status)

	// WithAppendQueryToMessageEndpoint must carry ?token= into the endpoint event,
	// otherwise every tool call below would come back 401.
	assert.Contains(t, c.endpoint, "token=s3cret")

	text := requireToolText(t, c.callTools("list_workspaces", map[string]any{}))
	assert.Contains(t, text, "workspaces")
}

func TestAuth_RequireTokenOff_AllowsAnonymous(t *testing.T) {
	srv := setupTestServer(t)
	srv.auth.Set("s3cret", false)
	base := startHTTPServer(t, srv)

	c, status := openSSE(t, base, "", "")
	require.Equal(t, http.StatusOK, status)

	text := requireToolText(t, c.callTools("list_workspaces", map[string]any{}))
	assert.Contains(t, text, "workspaces")
}

func TestTokenAuth_Allow(t *testing.T) {
	auth := NewTokenAuth("abc", true)

	newReq := func(url, header string) *http.Request {
		r, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		return r
	}

	assert.True(t, auth.Allow(newReq("http://x/sse", "Bearer abc")))
	assert.True(t, auth.Allow(newReq("http://x/sse", "BEARER abc")))
	assert.True(t, auth.Allow(newReq("http://x/sse?token=abc", "")))
	assert.False(t, auth.Allow(newReq("http://x/sse", "Bearer ab")))
	assert.False(t, auth.Allow(newReq("http://x/sse", "Basic abc")))
	assert.False(t, auth.Allow(newReq("http://x/sse", "")))

	// A required guard with no token is a broken setup, not an open door.
	blank := NewTokenAuth("", true)
	assert.False(t, blank.Allow(newReq("http://x/sse", "")))
	assert.False(t, blank.Allow(newReq("http://x/sse?token=", "")))
	assert.False(t, blank.Allow(newReq("http://x/sse", "Bearer anything")))
	assert.True(t, blank.Enabled())
}

func TestAuth_RequiredWithoutToken_RejectsEveryone(t *testing.T) {
	srv := setupTestServer(t)
	srv.auth.Set("", true)
	base := startHTTPServer(t, srv)

	_, status := openSSE(t, base, "", "")
	assert.Equal(t, http.StatusUnauthorized, status)

	_, status = openSSE(t, base, "token=", "")
	assert.Equal(t, http.StatusUnauthorized, status)

	_, status = openSSE(t, base, "", "Bearer guess")
	assert.Equal(t, http.StatusUnauthorized, status)
}

// A panic inside mcp-go's Shutdown used to skip the HTTP shutdown and leave the
// port bound; Stop must release it either way, and must survive being repeated.
func TestServer_Stop_ReleasesPort(t *testing.T) {
	srv := setupTestServer(t)
	base := startHTTPServer(t, srv)
	addr := strings.TrimPrefix(base, "http://")

	require.NoError(t, srv.Stop(context.Background()))
	require.NoError(t, srv.Stop(context.Background()), "a second Stop must not panic out")

	ln, err := net.Listen("tcp", addr)
	require.NoError(t, err, "port %s still bound after Stop", addr)
	_ = ln.Close()
}

// mcp-go closes the SSE session at quit, but a half-sent POST /message sits in the JSON
// decoder, so an unbounded Shutdown never returns and the app cannot exit.
func TestServer_Stop_BoundedWithAttachedClient(t *testing.T) {
	srv := setupTestServer(t)
	base := startHTTPServer(t, srv)
	addr := strings.TrimPrefix(base, "http://")

	c, status := openSSE(t, base, "", "")
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, c.endpoint)

	stalled, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = stalled.Close() }()
	_, err = fmt.Fprintf(stalled,
		"POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: 200\r\n\r\n{\"jsonrpc\":\"2.0\"",
		c.endpoint, addr)
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Stop(stopCtx) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop hung on a client that was still attached")
	}

	// The stranded handler must be dropped too: a session left alive keeps
	// serving tool calls against a server the app believes it has stopped.
	require.NoError(t, stalled.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = stalled.Read(make([]byte, 1))
	require.Error(t, err, "stalled connection survived Stop")
	var netErr net.Error
	require.False(t, errors.As(err, &netErr) && netErr.Timeout(),
		"stalled connection was left open after Stop")

	ln, err := net.Listen("tcp", addr)
	require.NoError(t, err, "port %s still bound after Stop", addr)
	_ = ln.Close()
}
