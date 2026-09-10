package requester

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tetiva-app/client/internal/domain"
)

func TestSpoolBody_UnderAndAtLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		max  int64
	}{
		{name: "under limit", body: "hello", max: 16},
		{name: "exactly at limit", body: "hello", max: 5},
		{name: "empty", body: "", max: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := spoolBody(strings.NewReader(tc.body), tc.max)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, readErr := io.ReadAll(r)
			if readErr != nil {
				t.Fatalf("read spooled: %v", readErr)
			}
			if string(got) != tc.body {
				t.Fatalf("expected %q, got %q", tc.body, got)
			}
		})
	}
}

func TestSpoolBody_OverLimit(t *testing.T) {
	_, err := spoolBody(strings.NewReader("hello world"), 5)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Fields["body"], "memory") {
		t.Fatalf("unexpected message: %q", ve.Fields["body"])
	}
}

func TestSpoolBody_Replayable(t *testing.T) {
	r, err := spoolBody(strings.NewReader("payload"), 64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash := hashPayload(r); hash != "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5" {
		t.Fatalf("unexpected payload hash: %s", hash)
	}
	// hashPayload must rewind: the request still has to send the body.
	got, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read after hashing: %v", readErr)
	}
	if string(got) != "payload" {
		t.Fatalf("expected body to be replayable, got %q", got)
	}
}
