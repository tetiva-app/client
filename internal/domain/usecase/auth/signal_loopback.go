package auth

import (
	"net/http"
	"time"
)

// signalPage is what the cabinet's redirect lands on. It carries no parameters:
// the tokens come from the poll, this is only a nudge and a place to close.
const signalPage = `<!doctype html><meta charset="utf-8"><title>Tetiva</title>` +
	`<body style="font:16px system-ui;padding:3rem;text-align:center">` +
	`You can close this tab. Return to Tetiva.</body>`

// signalLoopback answers the cabinet's redirect with the same hardening as loopback, but every query
// parameter is ignored: nothing secret travels this way.
type signalLoopback struct {
	*loopbackServer
	done chan struct{}
}

func startSignalLoopback(port string, opts FlowOptions) (*signalLoopback, error) {
	srv, err := listenLoopback(port, opts)
	if err != nil {
		return nil, err
	}

	l := &signalLoopback{loopbackServer: srv, done: make(chan struct{})}
	l.serve(func(w http.ResponseWriter, r *http.Request) {
		l.handle(w, r, opts.DrainWindow)
	})

	return l, nil
}

// signal is nil-safe: a flow that could not bind waits on a nil channel forever.
func (l *signalLoopback) signal() <-chan struct{} {
	if l == nil {
		return nil
	}

	return l.done
}

func (l *signalLoopback) handle(w http.ResponseWriter, r *http.Request, drain time.Duration) {
	if !l.accepts(w, r) {
		return
	}
	if !l.consume(drain) {
		http.Error(w, "this callback was already handled", http.StatusGone)

		return
	}

	writeCallbackPage(w, signalPage)
	close(l.done)
}
