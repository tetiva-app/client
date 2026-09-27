package instance

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	maxMessageBytes = 64 << 10
	maxReplyBytes   = 256
	connDeadline    = 2 * time.Second
	nonceBytes      = 32
	okReply         = "ok\n"
)

type instanceInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// A launch opens with helloMessage and sends ipcMessage only after the owner's proofMessage checks out.
type helloMessage struct {
	Nonce string `json:"nonce"`
}

type proofMessage struct {
	Proof string `json:"proof"`
}

type ipcMessage struct {
	Token string   `json:"token"`
	Args  []string `json:"args"`
}

func serve(lock *os.File, infoPath string, onArgs func([]string), listen func(network, address string) (net.Listener, error)) (*Instance, error) {
	ln, err := listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = unlockFile(lock)
		return nil, fmt.Errorf("listen: %w", err)
	}
	token, err := newToken()
	if err == nil {
		err = writeInfo(infoPath, ln.Addr().(*net.TCPAddr).Port, token)
	}
	if err != nil {
		_ = ln.Close()
		_ = unlockFile(lock)
		return nil, err
	}

	i := &Instance{lock: lock, listener: ln, token: token, infoPath: infoPath, onArgs: onArgs}
	i.wg.Add(1)
	go i.acceptLoop()
	return i, nil
}

func (i *Instance) acceptLoop() {
	defer i.wg.Done()
	for {
		conn, err := i.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			slog.Warn("instance: accept failed", "err", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		i.wg.Add(1)
		go func() {
			defer i.wg.Done()
			i.handle(conn)
		}()
	}
}

// A wrong token or a malformed message gets the connection closed without a reply.
func (i *Instance) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(connDeadline))

	dec := json.NewDecoder(io.LimitReader(conn, maxMessageBytes))
	var hello helloMessage
	if err := dec.Decode(&hello); err != nil {
		return
	}
	nonce, err := hex.DecodeString(hello.Nonce)
	if err != nil || len(nonce) != nonceBytes {
		return
	}
	reply, err := json.Marshal(proofMessage{Proof: hex.EncodeToString(proof(i.token, nonce))})
	if err != nil {
		return
	}
	if _, err := conn.Write(append(reply, '\n')); err != nil {
		return
	}

	var msg ipcMessage
	if err := dec.Decode(&msg); err != nil {
		return
	}
	if subtle.ConstantTimeCompare([]byte(msg.Token), []byte(i.token)) != 1 {
		slog.Warn("instance: rejected a launch message with a wrong token")
		return
	}
	if i.onArgs != nil {
		i.onArgs(msg.Args)
	}
	_, _ = io.WriteString(conn, okReply)
}

func forward(infoPath string, args []string, dialTimeout time.Duration) error {
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return err
	}
	var info instanceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return err
	}
	if info.Port <= 0 || info.Port > 65535 || info.Token == "" {
		return errors.New("instance.json is incomplete")
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(info.Port)), dialTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(connDeadline))

	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	hello, err := json.Marshal(helloMessage{Nonce: hex.EncodeToString(nonce)})
	if err != nil {
		return err
	}
	// No trailing newline here or below: unread bytes would turn the owner's FIN into an RST and drop the reply.
	if _, err := conn.Write(hello); err != nil {
		return err
	}
	replies := bufio.NewReader(io.LimitReader(conn, maxReplyBytes))
	line, err := replies.ReadString('\n')
	if err != nil {
		return err
	}
	var p proofMessage
	if err := json.Unmarshal([]byte(line), &p); err != nil {
		return err
	}
	got, err := hex.DecodeString(p.Proof)
	if err != nil || !hmac.Equal(got, proof(info.Token, nonce)) {
		return errors.New("the process on the port does not know the token")
	}

	raw, err := json.Marshal(ipcMessage{Token: info.Token, Args: args})
	if err != nil {
		return err
	}
	yieldForeground()
	if _, err := conn.Write(raw); err != nil {
		return err
	}
	reply, err := replies.ReadString('\n')
	if err != nil {
		return err
	}
	if reply != okReply {
		return errors.New("unexpected reply from the running instance")
	}
	return nil
}

func proof(token string, nonce []byte) []byte {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write(nonce)
	return mac.Sum(nil)
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func writeInfo(path string, port int, token string) error {
	data, err := json.Marshal(instanceInfo{Port: port, Token: token})
	if err != nil {
		return fmt.Errorf("instance.json: %w", err)
	}
	// CreateTemp opens the file 0600, so the token is never readable by others.
	tmp, err := os.CreateTemp(filepath.Dir(path), infoFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("instance.json: %w", err)
	}
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("instance.json: %w", err)
	}

	// Windows refuses to replace a file another process has open, e.g. a second launch reading the stale one.
	for attempt := 0; ; attempt++ {
		err = os.Rename(tmp.Name(), path)
		if err == nil || attempt == 9 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("instance.json: %w", err)
	}
	return nil
}
