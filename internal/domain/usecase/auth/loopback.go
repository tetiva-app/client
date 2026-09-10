package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// A browser redirect is one small GET, so nothing legitimate needs longer than
// this; the caps also keep a half-open client from holding the flow open.
const (
	loopbackReadHeaderTimeout = 5 * time.Second
	loopbackReadTimeout       = 10 * time.Second
	loopbackWriteTimeout      = 10 * time.Second
)

const callbackPath = "/callback"

const callbackPage = `<!doctype html><meta charset="utf-8"><title>Tetiva</title>` +
	`<body style="font:16px system-ui;padding:3rem;text-align:center">You can close this tab.</body>`

// callbackResult is the redirect the IdP sent the browser, already validated
// against the flow's state.
type callbackResult struct {
	Code           string
	Err            string
	ErrDescription string
}

// loopbackServer is the listener half both callbacks share: 127.0.0.1 only, the
// same timeouts, keep-alives off, the same one-shot-plus-drain discipline.
type loopbackServer struct {
	ln   net.Listener
	srv  *http.Server
	port int

	closeOnce sync.Once

	mu       sync.Mutex
	consumed bool
	drain    *time.Timer
}

// listenLoopback binds 127.0.0.1 only, so the redirect is never reachable from the network; port "0"
// asks the OS for a free one. The caller attaches its handler with serve.
func listenLoopback(port string, opts FlowOptions) (*loopbackServer, error) {
	const funcName = "auth.listenLoopback"

	ln, err := opts.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()

		return nil, fmt.Errorf("%s: unexpected listener address %v", funcName, ln.Addr())
	}

	return &loopbackServer{ln: ln, port: addr.Port}, nil
}

func (l *loopbackServer) serve(h http.HandlerFunc) {
	l.srv = &http.Server{
		Handler:           h,
		ReadHeaderTimeout: loopbackReadHeaderTimeout,
		ReadTimeout:       loopbackReadTimeout,
		WriteTimeout:      loopbackWriteTimeout,
	}
	l.srv.SetKeepAlivesEnabled(false)

	go func() { _ = l.srv.Serve(l.ln) }()
}

func (l *loopbackServer) redirectURI() string {
	return "http://127.0.0.1:" + strconv.Itoa(l.port) + callbackPath
}

func (l *loopbackServer) hostAllowed(host string) bool {
	port := strconv.Itoa(l.port)

	return host == net.JoinHostPort("127.0.0.1", port) || host == net.JoinHostPort("localhost", port)
}

// accepts is the shape both redirects must have; it answers the request itself
// when the shape is wrong, and neither flow ever learns about it.
func (l *loopbackServer) accepts(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method != http.MethodGet:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	case r.URL.Path != callbackPath:
		http.Error(w, "not found", http.StatusNotFound)
	case !l.hostAllowed(r.Host):
		http.Error(w, "bad request", http.StatusBadRequest)
	default:
		return true
	}

	return false
}

// consume flips the one-shot flag and arms the drain timer; false means the
// callback was already answered and this one must get 410.
func (l *loopbackServer) consume(drain time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.consumed {
		return false
	}
	l.consumed = true
	// The browser may retry the redirect; the window is short and bounded so
	// the port is free again for the next flow.
	l.drain = time.AfterFunc(drain, l.close)

	return true
}

// closeAfterDrain keeps a listener that answered a terminal callback alive for the rest of its drain
// window, so a browser that repeats the redirect gets 410 instead of a dead port.
func (l *loopbackServer) closeAfterDrain() {
	l.mu.Lock()
	draining := l.drain != nil
	l.mu.Unlock()

	if !draining {
		l.close()
	}
}

// close is immediate and idempotent: Shutdown would wait for a browser that has
// already been answered and may never close its connection.
func (l *loopbackServer) close() {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		if l.drain != nil {
			l.drain.Stop()
		}
		l.mu.Unlock()

		// The listener is closed directly as well: Serve tracks it on its own goroutine, so a close that beats
		// the tracking would free the port only once that goroutine runs — and the port is needed right now.
		_ = l.srv.Close()
		_ = l.ln.Close()
	})
}

// writeCallbackPage reaches the browser before the flow is told: the flow may close this listener the
// moment it has the result, and a half-written response would leave the user at a connection error.
func writeCallbackPage(w http.ResponseWriter, page string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, page)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

type loopback struct {
	*loopbackServer
	done chan callbackResult
}

func startLoopback(port, state string, opts FlowOptions) (*loopback, error) {
	srv, err := listenLoopback(port, opts)
	if err != nil {
		return nil, err
	}

	l := &loopback{loopbackServer: srv, done: make(chan callbackResult, 1)}
	l.serve(func(w http.ResponseWriter, r *http.Request) {
		l.handle(w, r, state, opts.DrainWindow)
	})

	return l, nil
}

func (l *loopback) handle(w http.ResponseWriter, r *http.Request, state string, drain time.Duration) {
	if !l.accepts(w, r) {
		return
	}

	res, err := parseCallback(r.URL.RawQuery, state)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)

		return
	}
	if !l.consume(drain) {
		http.Error(w, "this callback was already handled", http.StatusGone)

		return
	}

	writeCallbackPage(w, callbackPage)
	l.done <- res
}

// parseCallback enforces the whole redirect contract: one state, one of code or
// error, no repeated parameter. A query that fails it leaves the flow waiting.
func parseCallback(rawQuery, state string) (callbackResult, error) {
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return callbackResult{}, fmt.Errorf("auth: callback query: %w", err)
	}
	for _, key := range []string{"state", "code", "error", "error_description"} {
		if len(q[key]) > 1 {
			return callbackResult{}, fmt.Errorf("auth: callback repeated %s", key)
		}
	}
	if len(q["state"]) != 1 {
		return callbackResult{}, errors.New("auth: callback carried no state")
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
		return callbackResult{}, errors.New("auth: callback state does not match")
	}

	code, errCode := q.Get("code"), q.Get("error")
	if (code == "") == (errCode == "") {
		return callbackResult{}, errors.New("auth: callback needs exactly one of code or error")
	}

	return callbackResult{Code: code, Err: errCode, ErrDescription: q.Get("error_description")}, nil
}
