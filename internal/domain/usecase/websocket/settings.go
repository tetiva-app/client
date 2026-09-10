package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tetiva-app/client/internal/domain"
)

// SettingsVersion is the only document version writers produce.
const SettingsVersion = 1

// schemaHint is the reason reported for a body that is not a settings document at all.
const schemaHint = "must be a websocket settings document {version, pingIntervalSec, subprotocols, messages}"

// maxPingIntervalSec keeps the seconds-to-Duration conversion inside int64.
const maxPingIntervalSec = float64(math.MaxInt64 / int64(time.Second))

// SavedMessage.Data is the message text, or base64 for the binary format.
type SavedMessage struct {
	ID     string
	Name   string
	Format string // json | text | binary
	Data   string
}

// Settings is the WebSocket configuration kept as a JSON document in Request.Body.
type Settings struct {
	PingInterval time.Duration
	Subprotocols []string
	Messages     []SavedMessage
}

// ParseSettings is tolerant: anything malformed falls back to defaults, so an imported body still connects.
func ParseSettings(body string) Settings {
	s, _ := decodeSettings(body)
	return s
}

// ValidateSettings is the write boundary: an empty body, or a well-formed
// version 1 document. Unknown keys are allowed so older clients keep round-tripping.
func ValidateSettings(body string) error {
	if _, problem := decodeSettings(body); problem != "" {
		return &domain.ValidationError{Fields: map[string]string{"body": problem}}
	}
	return nil
}

// decodeSettings parses the document once: the settings salvaged from it, plus the
// first schema violation for the strict boundary ("" when the document is valid).
func decodeSettings(body string) (Settings, string) {
	var s Settings
	if strings.TrimSpace(body) == "" {
		return s, ""
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &doc); err != nil || doc == nil {
		return s, schemaHint
	}

	problem := ""
	fail := func(reason string) {
		if problem == "" {
			problem = reason
		}
	}

	var version int
	if raw, ok := jsonField(doc, "version"); !ok || json.Unmarshal(raw, &version) != nil || version != SettingsVersion {
		fail(fmt.Sprintf("version must be %d", SettingsVersion))
	}

	if raw, ok := jsonField(doc, "pingIntervalSec"); ok {
		var sec float64
		err := json.Unmarshal(raw, &sec)
		// Whole seconds only: a fractional value would arm a sub-second ticker.
		whole := err == nil && sec >= 0 && sec <= maxPingIntervalSec && sec == math.Trunc(sec)
		if !whole {
			fail("pingIntervalSec must be a whole number of seconds (0 disables the keepalive)")
		} else if sec >= 1 {
			s.PingInterval = time.Duration(sec) * time.Second
		}
	}

	if raw, ok := jsonField(doc, "subprotocols"); ok {
		var subs []string
		if err := json.Unmarshal(raw, &subs); err != nil {
			fail("subprotocols must be an array of strings")
		} else {
			s.Subprotocols = subs
		}
	}

	if raw, ok := jsonField(doc, "messages"); ok {
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			fail("messages must be an array of {id, name, format, data}")
		} else {
			for i, item := range items {
				m, keep, reason := decodeMessage(item)
				if reason != "" {
					fail(fmt.Sprintf("messages[%d]: %s", i, reason))
				}
				if keep {
					s.Messages = append(s.Messages, m)
				}
			}
		}
	}

	return s, problem
}

// decodeMessage returns what a tolerant read keeps plus the strict violation: a writer spells out all
// four fields, but an older document missing a name still reads back instead of vanishing from the UI.
func decodeMessage(item map[string]json.RawMessage) (m SavedMessage, keep bool, problem string) {
	fields := []struct {
		key string
		dst *string
	}{
		{"id", &m.ID},
		{"name", &m.Name},
		{"format", &m.Format},
		{"data", &m.Data},
	}
	for _, f := range fields {
		raw, ok := jsonField(item, f.key)
		if !ok {
			if problem == "" {
				problem = f.key + " is required"
			}
			continue
		}
		if err := json.Unmarshal(raw, f.dst); err != nil {
			return SavedMessage{}, false, f.key + " must be a string"
		}
	}

	if m.ID == "" {
		return SavedMessage{}, false, "id is required"
	}
	switch m.Format {
	case "json", "text", "binary":
	default:
		return SavedMessage{}, false, "format must be one of json, text, binary"
	}
	return m, true, problem
}

// jsonField reads a key, treating an explicit null as absent.
func jsonField(doc map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	raw, ok := doc[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, false
	}
	return raw, true
}
