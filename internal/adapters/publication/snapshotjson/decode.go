package snapshotjson

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

// Reasons a snapshot is refused; they reach the frontend as ResultError.reason.
const (
	ReasonTooLarge       = "SNAPSHOT_TOO_LARGE"
	ReasonInvalid        = "SNAPSHOT_INVALID"
	ReasonUpdateRequired = "UPDATE_REQUIRED"
)

// Server limits (spec §4.4): a document the server would reject is not imported either.
const (
	maxFolderDepth = 16
	maxItems       = 5000
)

// Decode is the one bounded reader for a snapshot from a file or a link: plain or gzip JSON, at most
// limit bytes once decompressed. Unknown fields are ignored; a newer version asks for an update.
func Decode(r io.Reader, limit int64) (*publication.Snapshot, error) {
	br := bufio.NewReader(r)
	var src io.Reader = br
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, invalid("the gzip stream is broken")
		}
		defer func() { _ = zr.Close() }()
		src = zr
	}

	data, err := io.ReadAll(io.LimitReader(src, limit+1))
	if err != nil {
		return nil, invalid("the gzip stream is broken")
	}
	if int64(len(data)) > limit {
		return nil, reasonError(ReasonTooLarge, fmt.Sprintf("larger than %d MiB", limit>>20))
	}

	var head struct {
		Format  string          `json:"format"`
		Version json.RawMessage `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, invalid("not a JSON object")
	}
	if head.Format != format {
		return nil, invalid("not a Tetiva collection snapshot")
	}
	var v int
	if err := json.Unmarshal(head.Version, &v); err != nil || v < 1 {
		return nil, invalid("version must be a positive integer")
	}
	if v != version {
		return nil, reasonError(ReasonUpdateRequired, fmt.Sprintf("snapshot version %d needs a newer Tetiva", v))
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	// Numbers inside auth fields (JWT claims) must survive the round trip digit for digit.
	dec.UseNumber()
	var doc jsonSnapshot
	if err := dec.Decode(&doc); err != nil {
		return nil, invalid(err.Error())
	}
	return fromJSON(&doc)
}

// Sniff tells a snapshot from other JSON by its top-level format key; gzip counts as a snapshot,
// the only gzip input an import expects. It stops at the key, which Marshal writes first.
func Sniff(data []byte) bool {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return true
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return false
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == "format" {
			var f string
			return dec.Decode(&f) == nil && f == format
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return false
		}
	}
	return false
}

func invalid(msg string) error {
	return reasonError(ReasonInvalid, msg)
}

func reasonError(reason, msg string) error {
	return &domain.ReasonError{Reason: reason, Err: &domain.ValidationError{Fields: map[string]string{"snapshot": msg}}}
}

func fromJSON(doc *jsonSnapshot) (*publication.Snapshot, error) {
	c := doc.Collection
	count := 0
	items, err := itemsFromJSON(c.Items, 1, &count)
	if err != nil {
		return nil, err
	}
	s := &publication.Snapshot{
		Generator: doc.Generator,
		Locale:    doc.Locale,
		Collection: publication.Root{
			Folder: publication.Folder{
				ID: c.ID, Name: c.Name, Description: c.Description,
				Auth: authFromJSON(c.Auth), Scripts: scriptsFromJSON(c.Scripts), Items: items,
			},
			GRPCMetadata: headersFromJSON(c.GRPCMetadata),
		},
	}
	if env := doc.Environment; env != nil {
		vars := make([]publication.Variable, 0, len(env.Variables))
		for _, v := range env.Variables {
			vars = append(vars, publication.Variable{Key: v.Key, Value: v.Value, Secret: v.Secret})
		}
		s.Environment = &publication.Environment{Name: env.Name, Variables: vars}
	}
	return s, nil
}

// itemsFromJSON counts folder depth from 1 at the top level, as the server validator does.
func itemsFromJSON(items []jsonItem, depth int, count *int) ([]publication.Item, error) {
	out := make([]publication.Item, 0, len(items))
	for i := range items {
		it := &items[i]
		*count++
		if *count > maxItems {
			return nil, invalid(fmt.Sprintf("more than %d folders and requests", maxItems))
		}
		switch it.Kind {
		case "folder":
			if depth > maxFolderDepth {
				return nil, invalid(fmt.Sprintf("folders nested deeper than %d levels", maxFolderDepth))
			}
			var children []jsonItem
			if it.Items != nil {
				children = *it.Items
			}
			sub, err := itemsFromJSON(children, depth+1, count)
			if err != nil {
				return nil, err
			}
			out = append(out, publication.Item{Folder: &publication.Folder{
				ID: it.ID, Name: it.Name, Description: it.Description,
				Auth: authFromJSON(it.Auth), Scripts: scriptsFromJSON(it.Scripts), Items: sub,
			}})
		case "request":
			out = append(out, publication.Item{Request: requestFromJSON(it)})
		default:
			return nil, invalid(fmt.Sprintf("item kind %q is neither folder nor request", it.Kind))
		}
	}
	return out, nil
}

func requestFromJSON(it *jsonItem) *publication.Request {
	r := &publication.Request{
		ID: it.ID, Name: it.Name, Description: it.Description, Protocol: entities.Protocol(it.Protocol),
		Auth: authFromJSON(it.Auth), Scripts: scriptsFromJSON(it.Scripts),
	}
	if h := it.HTTP; h != nil {
		fields := make([]publication.FormField, 0, len(h.Body.Fields))
		for _, f := range h.Body.Fields {
			fields = append(fields, publication.FormField{Key: f.Key, Value: f.Value, Type: f.Type, Enabled: f.Enabled})
		}
		r.HTTP = &publication.HTTPPart{
			Method: h.Method, URL: h.URL, Headers: headersFromJSON(h.Headers),
			Body: publication.Body{Type: h.Body.Type, Raw: h.Body.Raw, Fields: fields, FileName: h.Body.FileName},
		}
	}
	if g := it.GraphQL; g != nil {
		r.GraphQL = &publication.GraphQLPart{
			URL: g.URL, Headers: headersFromJSON(g.Headers), Query: g.Query, Variables: g.Variables, OperationName: g.OperationName,
		}
	}
	if g := it.GRPC; g != nil {
		r.GRPC = &publication.GRPCPart{
			Target: g.Target, Service: g.Service, Method: g.Method, Message: g.Message, Metadata: headersFromJSON(g.Metadata),
		}
	}
	if ws := it.WebSocket; ws != nil {
		messages := make([]publication.WSMessage, 0, len(ws.Messages))
		for _, m := range ws.Messages {
			messages = append(messages, publication.WSMessage{Name: m.Name, Format: m.Format, Data: m.Data})
		}
		r.WebSocket = &publication.WSPart{
			URL: ws.URL, Headers: headersFromJSON(ws.Headers), Subprotocols: append([]string{}, ws.Subprotocols...), Messages: messages,
		}
	}
	if it.Examples != nil {
		for _, e := range *it.Examples {
			r.Examples = append(r.Examples, publication.Example{
				ID: e.ID, Name: e.Name, Status: e.Status, StatusText: e.StatusText,
				Headers: headersFromJSON(e.Headers), Body: e.Body, ContentType: e.ContentType,
			})
		}
	}
	return r
}

func headersFromJSON(headers []jsonHeader) []publication.Header {
	out := make([]publication.Header, 0, len(headers))
	for _, h := range headers {
		out = append(out, publication.Header{Key: h.Key, Value: h.Value, Enabled: h.Enabled, Redacted: h.Redacted})
	}
	return out
}

func authFromJSON(a *jsonAuth) *publication.Auth {
	if a == nil {
		return nil
	}
	fields := a.Fields
	if fields == nil {
		fields = map[string]any{}
	}
	return &publication.Auth{Type: a.Type, Fields: fields, Redacted: append([]string{}, a.Redacted...)}
}

func scriptsFromJSON(s *jsonScripts) *publication.Scripts {
	if s == nil {
		return nil
	}
	return &publication.Scripts{Pre: s.Pre, Post: s.Post}
}
