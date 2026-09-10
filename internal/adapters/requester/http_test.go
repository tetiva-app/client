package requester

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestHTTPRequester_GET_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept header, got %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method:  entities.MethodGET,
		URL:     server.URL,
		Headers: map[string][]string{"Accept": {"application/json"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Body != `{"ok":true}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
	if resp.Size != 11 {
		t.Errorf("expected size 11, got %d", resp.Size)
	}
	if resp.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if resp.Protocol != entities.ProtocolHTTP {
		t.Errorf("expected protocol http, got %s", resp.Protocol)
	}
	ct, ok := resp.Headers["Content-Type"]
	if !ok || len(ct) == 0 || ct[0] != "application/json" {
		t.Errorf("expected Content-Type header, got %v", resp.Headers)
	}
}

func TestHTTPRequester_POST_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"name":"test"}` {
			t.Errorf("unexpected body: %s", string(body))
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method:  entities.MethodPOST,
		URL:     server.URL,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    `{"name":"test"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestHTTPRequester_ConnectionRefused(t *testing.T) {
	requester := NewHTTPRequester(nil)
	_, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    "http://127.0.0.1:1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "request failed") {
		t.Errorf("expected connection error, got: %v", err)
	}
}

func TestHTTPRequester_BinaryResponse(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
		_, _ = w.Write(pngHeader)
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsBinary {
		t.Error("expected IsBinary to be true")
	}
	if resp.BinaryPath == "" {
		t.Error("expected BinaryPath to be set")
	}
	if resp.Body != "" {
		t.Error("expected Body to be empty for binary response")
	}
	if resp.Size != int64(len(pngHeader)) {
		t.Errorf("expected size %d, got %d", len(pngHeader), resp.Size)
	}

	data, err := os.ReadFile(resp.BinaryPath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if !bytes.Equal(data, pngHeader) {
		t.Errorf("temp file content mismatch")
	}
	_ = os.Remove(resp.BinaryPath)
}

func TestHTTPRequester_XLSXResponse_IsBinary(t *testing.T) {
	// xlsx is a ZIP container; its first bytes are the local file header "PK\x03\x04".
	xlsxHeader := []byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="report.xlsx"`)
		w.WriteHeader(200)
		_, _ = w.Write(xlsxHeader)
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsBinary {
		t.Error("expected IsBinary to be true for xlsx")
	}
	if resp.Body != "" {
		t.Errorf("expected Body to be empty for binary response, got %q", resp.Body)
	}
	if resp.SuggestedFilename != "report.xlsx" {
		t.Errorf("expected SuggestedFilename %q, got %q", "report.xlsx", resp.SuggestedFilename)
	}
	if resp.BinaryPath != "" {
		_ = os.Remove(resp.BinaryPath)
	}
}

func TestHTTPRequester_AttachmentDisposition_IsBinary(t *testing.T) {
	// A textual Content-Type, but Content-Disposition: attachment signals a download.
	body := []byte("id,name\n1,Alice\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="data.csv"`)
		w.WriteHeader(200)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsBinary {
		t.Error("expected IsBinary to be true for attachment disposition")
	}
	if resp.SuggestedFilename != "data.csv" {
		t.Errorf("expected SuggestedFilename %q, got %q", "data.csv", resp.SuggestedFilename)
	}
	if resp.BinaryPath != "" {
		_ = os.Remove(resp.BinaryPath)
	}
}

func TestHTTPRequester_TextResponse_NotBinary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsBinary {
		t.Error("expected IsBinary to be false for JSON")
	}
	if resp.BinaryPath != "" {
		t.Error("expected BinaryPath to be empty for text response")
	}
	if resp.Body != `{"ok":true}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
}

func TestHTTPRequester_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	requester := NewHTTPRequester(nil)
	_, err := requester.Execute(ctx, request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPRequester_SigV4_SignsTheSentRequest(t *testing.T) {
	var gotAuth, gotDate, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotAuth = r.Header.Get("Authorization")
		gotDate = r.Header.Get("X-Amz-Date")
		w.WriteHeader(200)
	}))
	defer server.Close()

	requester := NewHTTPRequester(nil)
	resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method:  entities.MethodPOST,
		URL:     server.URL + "/things",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    `{"a":1}`,
		Auth: &request.RequestAuth{
			Type: entities.AuthTypeAWSSigV4,
			Fields: map[string]any{
				"accessKeyId": "AKIDEXAMPLE", "secretAccessKey": "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
				"region": "us-east-1", "service": "execute-api",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/") {
		t.Errorf("unexpected Authorization: %q", gotAuth)
	}
	if !strings.Contains(gotAuth, "SignedHeaders=content-type;host;x-amz-date") {
		t.Errorf("unexpected signed headers: %q", gotAuth)
	}
	if gotDate == "" {
		t.Error("expected X-Amz-Date on the wire")
	}
	if gotBody != `{"a":1}` {
		t.Errorf("body not sent: %q", gotBody)
	}
}

func TestHTTPRequester_SigV4_MissingCredentials(t *testing.T) {
	requester := NewHTTPRequester(nil)
	_, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
		Method: entities.MethodGET,
		URL:    "https://example.amazonaws.com/",
		Auth: &request.RequestAuth{
			Type:   entities.AuthTypeAWSSigV4,
			Fields: map[string]any{"accessKeyId": "AKIDEXAMPLE"},
		},
	})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestHTTPRequester_Redirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("landed"))
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	sigv4 := &request.RequestAuth{
		Type: entities.AuthTypeAWSSigV4,
		Fields: map[string]any{
			"accessKeyId": "AKIDEXAMPLE", "secretAccessKey": "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			"region": "us-east-1", "service": "service",
		},
	}
	tests := []struct {
		name       string
		auth       *request.RequestAuth
		wantStatus int
	}{
		{name: "no auth follows", auth: nil, wantStatus: 200},
		{name: "bearer follows", auth: &request.RequestAuth{Type: entities.AuthTypeBearer}, wantStatus: 200},
		{name: "sigv4 stops", auth: sigv4, wantStatus: 302},
		{name: "digest stops", auth: digestAuth("neo", "trinity"), wantStatus: 302},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requester := NewHTTPRequester(nil)
			resp, err := requester.Execute(context.Background(), request.HTTPExecuteRequest{
				Method: entities.MethodGET,
				URL:    redirector.URL,
				Auth:   tc.auth,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, resp.StatusCode)
			}
		})
	}
}
