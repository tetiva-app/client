package websocket

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain"
)

const fullSettingsDoc = `{
  "version": 1,
  "pingIntervalSec": 15,
  "subprotocols": ["graphql-ws", "json"],
  "messages": [
    {"id": "8b1f", "name": "Login", "format": "json", "data": "{\"op\":\"login\"}"},
    {"id": "9c2e", "name": "Ping", "format": "text", "data": "ping"},
    {"id": "a3d0", "name": "Blob", "format": "binary", "data": "aGVsbG8="}
  ]
}`

func TestParseSettings(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Settings
	}{
		{name: "empty", body: "", want: Settings{}},
		{name: "blank", body: "   \n", want: Settings{}},
		{name: "garbage", body: "not a document", want: Settings{}},
		{name: "not an object", body: `[1, 2]`, want: Settings{}},
		{name: "null", body: `null`, want: Settings{}},
		{
			name: "partial",
			body: `{"version":1,"pingIntervalSec":30}`,
			want: Settings{PingInterval: 30 * time.Second},
		},
		{
			name: "negative ping",
			body: `{"version":1,"pingIntervalSec":-5}`,
			want: Settings{},
		},
		{
			name: "ping seconds out of range",
			body: `{"version":1,"pingIntervalSec":1e30}`,
			want: Settings{},
		},
		{
			name: "sub-second ping is clamped to off",
			body: `{"version":1,"pingIntervalSec":1e-9}`,
			want: Settings{},
		},
		{
			name: "fractional ping is clamped to off",
			body: `{"version":1,"pingIntervalSec":1.5}`,
			want: Settings{},
		},
		{
			name: "unknown keys and unknown version keep known fields",
			body: `{"version":7,"pingIntervalSec":2,"colour":"red","subprotocols":["a"]}`,
			want: Settings{PingInterval: 2 * time.Second, Subprotocols: []string{"a"}},
		},
		{
			name: "invalid messages are skipped",
			body: `{"version":1,"messages":[{"id":"a","name":"A","format":"xml","data":"x"},{"id":"b","name":"B","format":"text","data":"y"}]}`,
			want: Settings{Messages: []SavedMessage{{ID: "b", Name: "B", Format: "text", Data: "y"}}},
		},
		{
			name: "message without a name or data reads with empty defaults",
			body: `{"version":1,"messages":[{"id":"a","format":"text"}]}`,
			want: Settings{Messages: []SavedMessage{{ID: "a", Format: "text"}}},
		},
		{
			name: "full document",
			body: fullSettingsDoc,
			want: Settings{
				PingInterval: 15 * time.Second,
				Subprotocols: []string{"graphql-ws", "json"},
				Messages: []SavedMessage{
					{ID: "8b1f", Name: "Login", Format: "json", Data: `{"op":"login"}`},
					{ID: "9c2e", Name: "Ping", Format: "text", Data: "ping"},
					{ID: "a3d0", Name: "Blob", Format: "binary", Data: "aGVsbG8="},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSettings(tt.body)
			if got.PingInterval != tt.want.PingInterval {
				t.Errorf("PingInterval: want %v, got %v", tt.want.PingInterval, got.PingInterval)
			}
			if len(got.Subprotocols) != len(tt.want.Subprotocols) {
				t.Fatalf("Subprotocols: want %v, got %v", tt.want.Subprotocols, got.Subprotocols)
			}
			for i, sub := range tt.want.Subprotocols {
				if got.Subprotocols[i] != sub {
					t.Errorf("Subprotocols[%d]: want %q, got %q", i, sub, got.Subprotocols[i])
				}
			}
			if len(got.Messages) != len(tt.want.Messages) {
				t.Fatalf("Messages: want %d, got %d (%+v)", len(tt.want.Messages), len(got.Messages), got.Messages)
			}
			for i, m := range tt.want.Messages {
				if got.Messages[i] != m {
					t.Errorf("Messages[%d]: want %+v, got %+v", i, m, got.Messages[i])
				}
			}
		})
	}
}

func TestValidateSettings_Accepts(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "blank", body: "  "},
		{name: "minimal", body: `{"version":1}`},
		{name: "full document", body: fullSettingsDoc},
		{name: "unknown top-level key", body: `{"version":1,"colour":"red"}`},
		{name: "null optional fields", body: `{"version":1,"pingIntervalSec":null,"subprotocols":null,"messages":null}`},
		{name: "zero ping", body: `{"version":1,"pingIntervalSec":0}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSettings(tt.body); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateSettings_Rejects(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "garbage", body: "not a document"},
		{name: "array", body: `[]`},
		{name: "json null", body: `null`},
		{name: "missing version", body: `{"pingIntervalSec":5}`},
		{name: "other version", body: `{"version":2}`},
		{name: "negative ping", body: `{"version":1,"pingIntervalSec":-1}`},
		{name: "ping not a number", body: `{"version":1,"pingIntervalSec":"30"}`},
		{name: "subprotocols not an array", body: `{"version":1,"subprotocols":"json"}`},
		{name: "subprotocol not a string", body: `{"version":1,"subprotocols":[1]}`},
		{name: "messages not an array", body: `{"version":1,"messages":{}}`},
		{name: "message format unknown", body: `{"version":1,"messages":[{"id":"a","name":"A","format":"xml","data":"x"}]}`},
		{name: "message id missing", body: `{"version":1,"messages":[{"name":"A","format":"text","data":"x"}]}`},
		{name: "message data not a string", body: `{"version":1,"messages":[{"id":"a","name":"A","format":"text","data":5}]}`},
		{name: "message name missing", body: `{"version":1,"messages":[{"id":"a","format":"text","data":"x"}]}`},
		{name: "message data missing", body: `{"version":1,"messages":[{"id":"a","name":"A","format":"text"}]}`},
		{name: "message format missing", body: `{"version":1,"messages":[{"id":"a","name":"A","data":"x"}]}`},
		{name: "message name not a string", body: `{"version":1,"messages":[{"id":"a","name":5,"format":"text","data":"x"}]}`},
		{name: "sub-second ping", body: `{"version":1,"pingIntervalSec":1e-9}`},
		{name: "fractional ping", body: `{"version":1,"pingIntervalSec":0.5}`},
		{name: "ping with a fraction above a second", body: `{"version":1,"pingIntervalSec":1.5}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSettings(tt.body)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			var verr *domain.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected *domain.ValidationError, got %T", err)
			}
			if verr.Fields["body"] == "" {
				t.Fatalf("expected a reason under the body field, got %v", verr.Fields)
			}
		})
	}
}

func TestValidateSettings_ReasonNamesTheSchema(t *testing.T) {
	err := ValidateSettings("not a document")
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *domain.ValidationError, got %T", err)
	}
	for _, key := range []string{"version", "pingIntervalSec", "subprotocols", "messages"} {
		if !strings.Contains(verr.Fields["body"], key) {
			t.Errorf("reason %q does not name %q", verr.Fields["body"], key)
		}
	}
}
