package instance

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const testLink = "tetiva://import?slug=petstore-api-k3f9x2qa"

var fastOptions = options{wait: 400 * time.Millisecond, retry: 50 * time.Millisecond, dial: 200 * time.Millisecond}

func readInfoFile(t *testing.T, dir string) instanceInfo {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, infoFileName))
	if err != nil {
		t.Fatal(err)
	}
	var info instanceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("instance.json: %v", err)
	}
	return info
}

const testNonce = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func expectedProof(t *testing.T, token string) string {
	t.Helper()

	nonce, err := hex.DecodeString(testNonce)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write(nonce)
	return hex.EncodeToString(mac.Sum(nil))
}

func send(t *testing.T, info instanceInfo, raw []byte) (proof, reply string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(info.Port)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(`{"nonce":"` + testNonce + `"}`)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("no proof after the nonce: %v", err)
	}
	var p struct {
		Proof string `json:"proof"`
	}
	if err := json.Unmarshal([]byte(line), &p); err != nil {
		t.Fatalf("proof %q: %v", line, err)
	}
	// A rejected write surfaces as the empty reply rejection tests expect.
	_, _ = conn.Write(raw)
	rest, _ := io.ReadAll(r)
	return p.Proof, string(rest)
}

func message(t *testing.T, token string, args []string) []byte {
	t.Helper()

	raw, err := json.Marshal(ipcMessage{Token: token, Args: args})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func acquireRecording(t *testing.T) (string, chan []string) {
	t.Helper()

	dir := t.TempDir()
	got := make(chan []string, 4)
	inst, err := Acquire(dir, nil, func(args []string) { got <- args })
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })
	return dir, got
}

func TestIPCAcceptsTheToken(t *testing.T) {
	dir, got := acquireRecording(t)
	info := readInfoFile(t, dir)
	if len(info.Token) != 64 {
		t.Fatalf("token length = %d, want 64 hex chars", len(info.Token))
	}

	proof, reply := send(t, info, message(t, info.Token, []string{"--flag", testLink}))

	if proof != expectedProof(t, info.Token) {
		t.Fatalf("proof = %q, want HMAC-SHA256 of the nonce keyed by the token", proof)
	}
	if reply != "ok\n" {
		t.Fatalf("reply = %q, want ok", reply)
	}
	select {
	case args := <-got:
		if len(args) != 2 || args[1] != testLink {
			t.Fatalf("onArgs got %q", args)
		}
	default:
		t.Fatal("onArgs not called before the reply")
	}
}

func TestIPCRejectsWrongToken(t *testing.T) {
	dir, got := acquireRecording(t)
	info := readInfoFile(t, dir)

	_, reply := send(t, info, message(t, strings.Repeat("0", 64), []string{testLink}))

	if reply != "" {
		t.Fatalf("reply = %q, want the connection closed without an answer", reply)
	}
	select {
	case args := <-got:
		t.Fatalf("onArgs called with %q for a wrong token", args)
	default:
	}
}

func TestIPCRejectsOversizedMessage(t *testing.T) {
	dir, got := acquireRecording(t)
	info := readInfoFile(t, dir)

	_, reply := send(t, info, message(t, info.Token, []string{strings.Repeat("a", maxMessageBytes)}))

	if reply != "" {
		t.Fatalf("reply = %q, want no answer to a message over the limit", reply)
	}
	select {
	case args := <-got:
		t.Fatalf("onArgs called for an oversized message (%d args)", len(args))
	default:
	}
}

func TestInProcessSecondAcquireForwards(t *testing.T) {
	dir, got := acquireRecording(t)

	inst, err := acquire(dir, []string{testLink}, nil, fastOptions)
	if err == nil {
		_ = inst.Close()
	}

	if !errors.Is(err, ErrForwarded) {
		t.Fatalf("err = %v, want ErrForwarded", err)
	}
	if args := <-got; len(args) != 1 || args[0] != testLink {
		t.Fatalf("onArgs got %q", args)
	}
}

func TestCloseReleasesLockAndRemovesFile(t *testing.T) {
	dir := t.TempDir()
	inst, err := Acquire(dir, nil, nil)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if err := inst.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, infoFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("instance.json after Close: %v", err)
	}
	again, err := acquire(dir, nil, nil, fastOptions)
	if err != nil {
		t.Fatalf("lock still held after Close: %v", err)
	}
	_ = again.Close()
}

func TestStopServingKeepsTheLockUntilClose(t *testing.T) {
	dir := t.TempDir()
	got := make(chan []string, 1)
	inst, err := Acquire(dir, nil, func(args []string) { got <- args })
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	if err := inst.StopServing(); err != nil {
		t.Fatalf("StopServing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, infoFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("instance.json after StopServing: %v", err)
	}

	type result struct {
		inst *Instance
		err  error
	}
	second := make(chan result, 1)
	go func() {
		next, err := acquire(dir, []string{testLink}, nil, options{wait: 5 * time.Second, retry: 20 * time.Millisecond, dial: 200 * time.Millisecond})
		second <- result{next, err}
	}()

	select {
	case r := <-second:
		if r.inst != nil {
			_ = r.inst.Close()
		}
		t.Fatalf("second acquire finished while the first still shuts down: %v", r.err)
	case <-time.After(300 * time.Millisecond):
	}

	if err := inst.Close(); err != nil {
		t.Fatalf("Close after StopServing: %v", err)
	}
	select {
	case r := <-second:
		if r.err != nil {
			t.Fatalf("second acquire after Close: %v, want it to take over", r.err)
		}
		_ = r.inst.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("second acquire did not take over after Close")
	}
	select {
	case args := <-got:
		t.Fatalf("stopped instance received %q", args)
	default:
	}
}

func impostor(t *testing.T) (port int, heard func() string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var mu sync.Mutex
	var log strings.Builder
	record := func(b []byte) string {
		mu.Lock()
		defer mu.Unlock()
		log.Write(b)
		return string(b)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = conn.SetDeadline(time.Now().Add(time.Second))
				buf := make([]byte, 4096)
				n, _ := conn.Read(buf)
				if strings.Contains(record(buf[:n]), `"token"`) {
					_, _ = io.WriteString(conn, "ok\n")
					return
				}
				_, _ = io.WriteString(conn, `{"proof":"`+strings.Repeat("ab", 32)+`"}`+"\n")
				n, _ = conn.Read(buf)
				record(buf[:n])
				_, _ = io.WriteString(conn, "ok\n")
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() string {
		mu.Lock()
		defer mu.Unlock()
		return log.String()
	}
}

func TestForwardDoesNotTrustAnImpostorOnThePort(t *testing.T) {
	dir := t.TempDir()
	lock, err := lockFile(filepath.Join(dir, lockFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unlockFile(lock) })
	port, heard := impostor(t)
	token := strings.Repeat("7", 64)
	if err := writeInfo(filepath.Join(dir, infoFileName), port, token); err != nil {
		t.Fatal(err)
	}

	inst, err := acquire(dir, []string{testLink}, nil, fastOptions)
	if err == nil {
		_ = inst.Close()
	}

	if !errors.Is(err, ErrNotResponding) {
		t.Fatalf("err = %v, want ErrNotResponding: an answer without the proof is no delivery", err)
	}
	if got := heard(); strings.Contains(got, "slug=") || strings.Contains(got, token) {
		t.Fatalf("the impostor heard %q", got)
	}
}

func TestIPCIgnoresAMalformedNonce(t *testing.T) {
	dir, got := acquireRecording(t)
	info := readInfoFile(t, dir)

	for _, hello := range []string{`{"nonce":"abc"}`, `{"nonce":"` + strings.Repeat("zz", 32) + `"}`, `{}`} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(info.Port)), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = conn.Write([]byte(hello))
		reply, _ := io.ReadAll(conn)
		_ = conn.Close()

		if len(reply) != 0 {
			t.Fatalf("hello %s got %q, want the connection closed without a proof", hello, reply)
		}
	}
	select {
	case args := <-got:
		t.Fatalf("onArgs called with %q", args)
	default:
	}
}

func TestStaleInstanceFileGoesRightAfterTheLock(t *testing.T) {
	dir := t.TempDir()
	infoPath := filepath.Join(dir, infoFileName)
	if err := writeInfo(infoPath, 1, "stale-token"); err != nil {
		t.Fatal(err)
	}
	staleAtListen := false
	opt := fastOptions
	opt.listen = func(network, address string) (net.Listener, error) {
		_, err := os.Stat(infoPath)
		staleAtListen = err == nil
		return net.Listen(network, address)
	}

	inst, err := acquire(dir, nil, nil, opt)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	if staleAtListen {
		t.Fatal("the dead owner's instance.json was still there while the new owner set up its port")
	}
}

type closeRecorder struct {
	net.Listener
	onClose func()
}

func (l *closeRecorder) Close() error {
	l.onClose()
	return l.Listener.Close()
}

func TestInstanceFileGoesBeforeTheListenerCloses(t *testing.T) {
	dir := t.TempDir()
	infoPath := filepath.Join(dir, infoFileName)
	var once sync.Once
	fileAtClose := false
	opt := fastOptions
	opt.listen = func(network, address string) (net.Listener, error) {
		ln, err := net.Listen(network, address)
		if err != nil {
			return nil, err
		}
		return &closeRecorder{Listener: ln, onClose: func() {
			once.Do(func() {
				_, err := os.Stat(infoPath)
				fileAtClose = err == nil
			})
		}}, nil
	}
	inst, err := acquire(dir, nil, nil, opt)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	if err := inst.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if fileAtClose {
		t.Fatal("instance.json still named the port when the listener closed")
	}
}
