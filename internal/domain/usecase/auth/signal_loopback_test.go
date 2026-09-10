package auth

import (
	"errors"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func newTestSignalLoopback(t *testing.T, drain time.Duration) *signalLoopback {
	t.Helper()

	lb, err := startSignalLoopback("0", FlowOptions{DrainWindow: drain}.withDefaults())
	if err != nil {
		t.Fatalf("startSignalLoopback: %v", err)
	}
	t.Cleanup(lb.close)

	return lb
}

func portIsFree(t *testing.T, port int) bool {
	t.Helper()

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()

	return true
}

func TestSignalLoopbackBindsAFreeLoopbackPort(t *testing.T) {
	lb := newTestSignalLoopback(t, time.Second)

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

func TestSignalLoopbackWakesTheFlowAndThenAnswers410(t *testing.T) {
	// A drain window the second request cannot miss, however loaded the machine is.
	lb := newTestSignalLoopback(t, 30*time.Second)

	if resp := callbackGet(t, lb.redirectURI()); resp.StatusCode != http.StatusOK {
		t.Fatalf("redirect: status = %d", resp.StatusCode)
	}
	select {
	case <-lb.signal():
	case <-time.After(time.Second):
		t.Fatal("the redirect did not wake the flow")
	}

	if resp := callbackGet(t, lb.redirectURI()); resp.StatusCode != http.StatusGone {
		t.Errorf("repeated redirect inside the drain window: status = %d, want 410", resp.StatusCode)
	}
}

func TestSignalLoopbackIgnoresEveryQueryParameter(t *testing.T) {
	lb := newTestSignalLoopback(t, time.Second)

	target := lb.redirectURI() + "?code=x&state=y&foo=1&foo=2"
	if resp := callbackGet(t, target); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	select {
	case <-lb.signal():
	case <-time.After(time.Second):
		t.Fatal("a redirect carrying parameters did not wake the flow")
	}
}

func TestSignalLoopbackRejectsWhatIsNotItsRedirectAndKeepsWaiting(t *testing.T) {
	lb := newTestSignalLoopback(t, time.Second)
	base := lb.redirectURI()

	tests := []struct {
		name    string
		request func() *http.Request
		want    int
	}{
		{"post", func() *http.Request {
			req, _ := http.NewRequest(http.MethodPost, base, nil)

			return req
		}, http.StatusMethodNotAllowed},
		{"other path", func() *http.Request {
			req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(lb.port)+"/other", nil)

			return req
		}, http.StatusNotFound},
		{"foreign host header", func() *http.Request {
			req, _ := http.NewRequest(http.MethodGet, base, nil)
			req.Host = "attacker.example"

			return req
		}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := sendCallback(t, tt.request())
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
			select {
			case <-lb.signal():
				t.Fatal("a rejected request woke the flow")
			default:
			}
		})
	}

	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Host = net.JoinHostPort("localhost", strconv.Itoa(lb.port))
	if resp := sendCallback(t, req); resp.StatusCode != http.StatusOK {
		t.Fatalf("localhost host: status = %d", resp.StatusCode)
	}
	select {
	case <-lb.signal():
	case <-time.After(time.Second):
		t.Fatal("the valid redirect was not delivered")
	}
}

func TestSignalLoopbackReleasesItsPortAfterTheDrainWindow(t *testing.T) {
	lb := newTestSignalLoopback(t, 20*time.Millisecond)

	if resp := callbackGet(t, lb.redirectURI()); resp.StatusCode != http.StatusOK {
		t.Fatalf("redirect: status = %d", resp.StatusCode)
	}
	<-lb.signal()
	lb.closeAfterDrain()

	waitFor(t, "the port to be free again", func() bool { return portIsFree(t, lb.port) })
}

func TestSignalLoopbackCloseIsIdempotent(t *testing.T) {
	lb := newTestSignalLoopback(t, time.Second)

	lb.close()
	lb.close()
	lb.closeAfterDrain()

	waitFor(t, "the port to be free again", func() bool { return portIsFree(t, lb.port) })
}

func TestTheSignalOfAnUnboundLoopbackNeverFires(t *testing.T) {
	var lb *signalLoopback

	if ch := lb.signal(); ch != nil {
		t.Fatalf("signal() = %v, want nil", ch)
	}
	select {
	case <-lb.signal():
		t.Fatal("a loopback that never bound signalled")
	default:
	}
}

func TestBothLoopbacksReportAFailedBind(t *testing.T) {
	boom := errors.New("boom")
	opts := FlowOptions{Listen: func(string, string) (net.Listener, error) {
		return nil, boom
	}}.withDefaults()

	if _, err := startSignalLoopback("0", opts); !errors.Is(err, boom) {
		t.Errorf("startSignalLoopback error = %v, want %v", err, boom)
	}
	if _, err := startLoopback("0", testState, opts); !errors.Is(err, boom) {
		t.Errorf("startLoopback error = %v, want %v", err, boom)
	}
}

func TestFlowOptionsDefaultTheListener(t *testing.T) {
	listen := FlowOptions{}.withDefaults().Listen
	if listen == nil {
		t.Fatal("withDefaults left Listen nil")
	}
	if reflect.ValueOf(listen).Pointer() != reflect.ValueOf(net.Listen).Pointer() {
		t.Error("withDefaults did not substitute net.Listen")
	}
}
