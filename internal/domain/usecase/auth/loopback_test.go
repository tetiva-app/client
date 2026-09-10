package auth

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"
)

const testState = "the-state"

func newTestLoopback(t *testing.T, drain time.Duration) *loopback {
	t.Helper()

	lb, err := startLoopback("0", testState, FlowOptions{DrainWindow: drain}.withDefaults())
	if err != nil {
		t.Fatalf("startLoopback: %v", err)
	}
	t.Cleanup(lb.close)

	return lb
}

func sendCallback(t *testing.T, req *http.Request) *http.Response {
	t.Helper()

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

func callbackGet(t *testing.T, target string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	return sendCallback(t, req)
}

func TestLoopbackRejectsBadCallbacksAndKeepsWaiting(t *testing.T) {
	lb := newTestLoopback(t, time.Second)
	base := lb.redirectURI()

	tests := []struct {
		name    string
		request func(t *testing.T) *http.Request
		want    int
	}{
		{"post", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodPost, base+"?code=c&state="+testState, nil)

			return req
		}, http.StatusMethodNotAllowed},
		{"other path", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(lb.port)+"/other?code=c&state="+testState, nil)

			return req
		}, http.StatusNotFound},
		{"foreign host header", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=c&state="+testState, nil)
			req.Host = "attacker.example"

			return req
		}, http.StatusBadRequest},
		{"no state", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=c", nil)

			return req
		}, http.StatusBadRequest},
		{"wrong state", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=c&state=other", nil)

			return req
		}, http.StatusBadRequest},
		{"duplicate state", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=c&state="+testState+"&state="+testState, nil)

			return req
		}, http.StatusBadRequest},
		{"duplicate code", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=c&code=c2&state="+testState, nil)

			return req
		}, http.StatusBadRequest},
		{"duplicate error", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?error=a&error=b&state="+testState, nil)

			return req
		}, http.StatusBadRequest},
		{"duplicate error_description", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?error=a&error_description=x&error_description=y&state="+testState, nil)

			return req
		}, http.StatusBadRequest},
		{"code and error", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=c&error=denied&state="+testState, nil)

			return req
		}, http.StatusBadRequest},
		{"neither code nor error", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?state="+testState, nil)

			return req
		}, http.StatusBadRequest},
		{"empty code", func(t *testing.T) *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base+"?code=&state="+testState, nil)

			return req
		}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := sendCallback(t, tt.request(t))
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
			select {
			case res := <-lb.done:
				t.Fatalf("a rejected callback was delivered: %+v", res)
			default:
			}
		})
	}

	// The listener the IdP was told about is still the one waiting.
	req, _ := http.NewRequest(http.MethodGet, lb.redirectURI()+"?code=good&state="+testState, nil)
	req.Host = net.JoinHostPort("localhost", strconv.Itoa(lb.port))
	if resp := sendCallback(t, req); resp.StatusCode != http.StatusOK {
		t.Fatalf("localhost host: status = %d", resp.StatusCode)
	}
	select {
	case res := <-lb.done:
		if res.Code != "good" {
			t.Errorf("delivered %+v", res)
		}
	case <-time.After(time.Second):
		t.Fatal("the valid callback was not delivered")
	}
}

func TestLoopbackDeliversAnErrorRedirect(t *testing.T) {
	lb := newTestLoopback(t, time.Second)

	query := url.Values{"error": {"access_denied"}, "error_description": {"user said no"}, "state": {testState}}
	if resp := callbackGet(t, lb.redirectURI()+"?"+query.Encode()); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	res := <-lb.done
	if res.Err != "access_denied" || res.ErrDescription != "user said no" || res.Code != "" {
		t.Errorf("delivered %+v", res)
	}
}

func TestLoopbackAnswers410WhileItDrains(t *testing.T) {
	// A drain window the second request cannot miss, however loaded the machine is.
	lb := newTestLoopback(t, 30*time.Second)
	target := lb.redirectURI() + "?code=good&state=" + testState

	if resp := callbackGet(t, target); resp.StatusCode != http.StatusOK {
		t.Fatalf("first callback: status = %d", resp.StatusCode)
	}
	<-lb.done

	if resp := callbackGet(t, target); resp.StatusCode != http.StatusGone {
		t.Errorf("duplicate callback inside the drain window: status = %d, want 410", resp.StatusCode)
	}
}

func TestLoopbackClosesAfterItsDrainWindow(t *testing.T) {
	lb := newTestLoopback(t, 20*time.Millisecond)
	target := lb.redirectURI() + "?code=good&state=" + testState

	if resp := callbackGet(t, target); resp.StatusCode != http.StatusOK {
		t.Fatalf("first callback: status = %d", resp.StatusCode)
	}
	<-lb.done

	waitFor(t, "the listener to stop answering", func() bool {
		resp, err := (&http.Client{Timeout: time.Second}).Get(target)
		if err != nil {
			return true
		}
		_ = resp.Body.Close()

		return false
	})
}

func TestLoopbackCloseIsNotHeldByAHalfOpenClient(t *testing.T) {
	lb := newTestLoopback(t, time.Second)

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(lb.port)), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	closed := make(chan struct{})
	go func() {
		lb.close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close waited for a client that sent nothing")
	}
}

func TestLoopbackBindsLoopbackOnly(t *testing.T) {
	lb := newTestLoopback(t, time.Second)

	host, _, err := net.SplitHostPort(lb.ln.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("listening on %q, want 127.0.0.1", host)
	}
	if want := "http://127.0.0.1:" + strconv.Itoa(lb.port) + "/callback"; lb.redirectURI() != want {
		t.Errorf("redirectURI() = %q, want %q", lb.redirectURI(), want)
	}
}
