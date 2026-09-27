// Package snapshotjson is the JSON form of the collection snapshot (CollectionSnapshot v1).
package snapshotjson

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"

	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

const (
	format  = "tetiva.collection-snapshot"
	version = 1
)

// Struct field order is the canonical key order; encoding/json sorts map keys.
type jsonSnapshot struct {
	Format      string           `json:"format"`
	Version     int              `json:"version"`
	Generator   string           `json:"generator"`
	Locale      string           `json:"locale"`
	Collection  jsonCollection   `json:"collection"`
	Environment *jsonEnvironment `json:"environment"`
}

type jsonCollection struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Auth         *jsonAuth    `json:"auth"`
	Scripts      *jsonScripts `json:"scripts"`
	GRPCMetadata []jsonHeader `json:"grpcMetadata"`
	Items        []jsonItem   `json:"items"`
}

type jsonItem struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Protocol    string         `json:"protocol,omitempty"`
	HTTP        *jsonHTTP      `json:"http,omitempty"`
	GraphQL     *jsonGraphQL   `json:"graphql,omitempty"`
	GRPC        *jsonGRPC      `json:"grpc,omitempty"`
	WebSocket   *jsonWS        `json:"websocket,omitempty"`
	Auth        *jsonAuth      `json:"auth"`
	Scripts     *jsonScripts   `json:"scripts"`
	Items       *[]jsonItem    `json:"items,omitempty"`
	Examples    *[]jsonExample `json:"examples,omitempty"`
}

type jsonHTTP struct {
	Method  string       `json:"method"`
	URL     string       `json:"url"`
	Headers []jsonHeader `json:"headers"`
	Body    jsonBody     `json:"body"`
}

type jsonGraphQL struct {
	URL           string       `json:"url"`
	Headers       []jsonHeader `json:"headers"`
	Query         string       `json:"query"`
	Variables     string       `json:"variables"`
	OperationName string       `json:"operationName"`
}

type jsonGRPC struct {
	Target   string       `json:"target"`
	Service  string       `json:"service"`
	Method   string       `json:"method"`
	Message  string       `json:"message"`
	Metadata []jsonHeader `json:"metadata"`
}

type jsonWS struct {
	URL          string          `json:"url"`
	Headers      []jsonHeader    `json:"headers"`
	Subprotocols []string        `json:"subprotocols"`
	Messages     []jsonWSMessage `json:"messages"`
}

type jsonWSMessage struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Data   string `json:"data"`
}

type jsonHeader struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Enabled  bool   `json:"enabled"`
	Redacted bool   `json:"redacted"`
}

type jsonBody struct {
	Type     string          `json:"type"`
	Raw      string          `json:"raw"`
	Fields   []jsonFormField `json:"fields"`
	FileName string          `json:"fileName"`
}

type jsonFormField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type jsonAuth struct {
	Type     string         `json:"type"`
	Fields   map[string]any `json:"fields"`
	Redacted []string       `json:"redacted"`
}

type jsonScripts struct {
	Pre  string `json:"pre"`
	Post string `json:"post"`
}

type jsonExample struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Status      int          `json:"status"`
	StatusText  string       `json:"statusText"`
	Headers     []jsonHeader `json:"headers"`
	Body        string       `json:"body"`
	ContentType string       `json:"contentType"`
}

type jsonEnvironment struct {
	Name      string         `json:"name"`
	Variables []jsonVariable `json:"variables"`
}

type jsonVariable struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// Marshal writes the canonical form: [] never null, no HTML escaping, no trailing newline.
func Marshal(s *publication.Snapshot) ([]byte, error) {
	const funcName = "snapshotjson.Marshal"

	out, err := encode(toJSON(s))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return out, nil
}

func Gzip(b []byte) ([]byte, error) {
	const funcName = "snapshotjson.Gzip"

	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if _, err := zw.Write(b); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return buf.Bytes(), nil
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func toJSON(s *publication.Snapshot) jsonSnapshot {
	root := s.Collection
	out := jsonSnapshot{
		Format:    format,
		Version:   version,
		Generator: s.Generator,
		Locale:    s.Locale,
		Collection: jsonCollection{
			ID: root.ID, Name: root.Name, Description: root.Description,
			Auth: authToJSON(root.Auth), Scripts: scriptsToJSON(root.Scripts),
			GRPCMetadata: headersToJSON(root.GRPCMetadata), Items: itemsToJSON(root.Items),
		},
	}
	if env := s.Environment; env != nil {
		vars := make([]jsonVariable, 0, len(env.Variables))
		for _, v := range env.Variables {
			vars = append(vars, jsonVariable{Key: v.Key, Value: v.Value, Secret: v.Secret})
		}
		out.Environment = &jsonEnvironment{Name: env.Name, Variables: vars}
	}
	return out
}

func itemsToJSON(items []publication.Item) []jsonItem {
	out := make([]jsonItem, 0, len(items))
	for _, it := range items {
		switch {
		case it.Folder != nil:
			f := it.Folder
			children := itemsToJSON(f.Items)
			out = append(out, jsonItem{
				Kind: "folder", ID: f.ID, Name: f.Name, Description: f.Description,
				Auth: authToJSON(f.Auth), Scripts: scriptsToJSON(f.Scripts), Items: &children,
			})
		case it.Request != nil:
			out = append(out, requestToJSON(it.Request))
		}
	}
	return out
}

func requestToJSON(r *publication.Request) jsonItem {
	examples := make([]jsonExample, 0, len(r.Examples))
	for _, e := range r.Examples {
		examples = append(examples, jsonExample{
			ID: e.ID, Name: e.Name, Status: e.Status, StatusText: e.StatusText,
			Headers: headersToJSON(e.Headers), Body: e.Body, ContentType: e.ContentType,
		})
	}
	out := jsonItem{
		Kind: "request", ID: r.ID, Name: r.Name, Description: r.Description, Protocol: string(r.Protocol),
		Auth: authToJSON(r.Auth), Scripts: scriptsToJSON(r.Scripts), Examples: &examples,
	}
	if h := r.HTTP; h != nil {
		fields := make([]jsonFormField, 0, len(h.Body.Fields))
		for _, f := range h.Body.Fields {
			fields = append(fields, jsonFormField{Key: f.Key, Value: f.Value, Type: f.Type, Enabled: f.Enabled})
		}
		out.HTTP = &jsonHTTP{
			Method: h.Method, URL: h.URL, Headers: headersToJSON(h.Headers),
			Body: jsonBody{Type: h.Body.Type, Raw: h.Body.Raw, Fields: fields, FileName: h.Body.FileName},
		}
	}
	if g := r.GraphQL; g != nil {
		out.GraphQL = &jsonGraphQL{URL: g.URL, Headers: headersToJSON(g.Headers), Query: g.Query, Variables: g.Variables, OperationName: g.OperationName}
	}
	if g := r.GRPC; g != nil {
		out.GRPC = &jsonGRPC{Target: g.Target, Service: g.Service, Method: g.Method, Message: g.Message, Metadata: headersToJSON(g.Metadata)}
	}
	if ws := r.WebSocket; ws != nil {
		messages := make([]jsonWSMessage, 0, len(ws.Messages))
		for _, m := range ws.Messages {
			messages = append(messages, jsonWSMessage{Name: m.Name, Format: m.Format, Data: m.Data})
		}
		subprotocols := append(make([]string, 0, len(ws.Subprotocols)), ws.Subprotocols...)
		out.WebSocket = &jsonWS{URL: ws.URL, Headers: headersToJSON(ws.Headers), Subprotocols: subprotocols, Messages: messages}
	}
	return out
}

func headersToJSON(headers []publication.Header) []jsonHeader {
	out := make([]jsonHeader, 0, len(headers))
	for _, h := range headers {
		out = append(out, jsonHeader{Key: h.Key, Value: h.Value, Enabled: h.Enabled, Redacted: h.Redacted})
	}
	return out
}

func authToJSON(a *publication.Auth) *jsonAuth {
	if a == nil {
		return nil
	}
	fields := a.Fields
	if fields == nil {
		fields = map[string]any{}
	}
	return &jsonAuth{Type: a.Type, Fields: fields, Redacted: append(make([]string, 0, len(a.Redacted)), a.Redacted...)}
}

func scriptsToJSON(s *publication.Scripts) *jsonScripts {
	if s == nil {
		return nil
	}
	return &jsonScripts{Pre: s.Pre, Post: s.Post}
}
