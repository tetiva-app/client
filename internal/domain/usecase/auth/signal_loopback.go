package auth

import (
	"net/http"
	"time"
)

// signalLoopback answers the cabinet's redirect with the same hardening as loopback, but every query
// parameter is ignored: nothing secret travels this way.
type signalLoopback struct {
	*loopbackServer
	done chan struct{}
}

func startSignalLoopback(port, locale string, opts FlowOptions) (*signalLoopback, error) {
	srv, err := listenLoopback(port, locale, opts)
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
		writeCallbackPage(w, http.StatusGone, renderCallbackPage(l.locale, pageSignInAlreadyHandled, ""))

		return
	}

	writeCallbackPage(w, http.StatusOK, renderCallbackPage(l.locale, pageSignedIn, ""))
	close(l.done)
}
