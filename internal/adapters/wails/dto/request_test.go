package dto

import (
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestResponseToExecuteDTO_TextResponse(t *testing.T) {
	resp := &entities.Response{
		StatusCode: 200,
		StatusText: "200 OK",
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       `{"ok":true}`,
		Size:       11,
		Duration:   142 * time.Millisecond,
		Protocol:   entities.ProtocolHTTP,
	}

	dto := ResponseToExecuteDTO(resp)

	if dto.StatusCode != 200 {
		t.Errorf("expected StatusCode 200, got %d", dto.StatusCode)
	}
	if dto.Body != `{"ok":true}` {
		t.Errorf("expected body, got %q", dto.Body)
	}
	if dto.DurationMs != 142 {
		t.Errorf("expected 142ms, got %d", dto.DurationMs)
	}
	if dto.IsBinary {
		t.Error("expected IsBinary to be false for text response")
	}
	if dto.BinaryPath != "" {
		t.Error("expected BinaryPath to be empty for text response")
	}
}

func TestResponseToExecuteDTO_BinaryResponse(t *testing.T) {
	resp := &entities.Response{
		StatusCode: 200,
		StatusText: "200 OK",
		Headers:    map[string][]string{"Content-Type": {"image/png"}},
		Body:       "",
		Size:       4096,
		Duration:   89 * time.Millisecond,
		Protocol:   entities.ProtocolHTTP,
		IsBinary:   true,
		BinaryPath: "/tmp/gopher-response-12345",
	}

	dto := ResponseToExecuteDTO(resp)

	if !dto.IsBinary {
		t.Error("expected IsBinary to be true")
	}
	if dto.BinaryPath != "/tmp/gopher-response-12345" {
		t.Errorf("expected BinaryPath, got %q", dto.BinaryPath)
	}
	if dto.Body != "" {
		t.Error("expected empty body for binary response")
	}
	if dto.Size != 4096 {
		t.Errorf("expected size 4096, got %d", dto.Size)
	}
}

func TestResponseToExecuteDTO_NilHeaders(t *testing.T) {
	resp := &entities.Response{
		StatusCode: 200,
		StatusText: "200 OK",
		Headers:    nil,
		Body:       "ok",
		Size:       2,
		Duration:   10 * time.Millisecond,
		Protocol:   entities.ProtocolHTTP,
	}

	dto := ResponseToExecuteDTO(resp)

	if dto.Headers == nil {
		t.Error("expected non-nil headers map")
	}
}
