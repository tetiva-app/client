// Package publication builds the collection snapshot a published page is rendered from: the live
// tree of one root collection, scrubbed of secrets, scanned for leaked credentials, with opaque ids.
package publication

import "github.com/tetiva-app/client/internal/domain/entities"

// Snapshot mirrors CollectionSnapshot v1 (proto/snapshot/collection-snapshot.v1.schema.json);
// the JSON form lives in adapters/publication/snapshotjson.
type Snapshot struct {
	Generator   string
	Locale      string
	Collection  Root
	Environment *Environment
}

// Root is the only level that carries grpcMetadata; requests get the merged set in GRPCPart.
type Root struct {
	Folder
	GRPCMetadata []Header
}

// Folder.Auth nil is AuthTypeNone at that level: requests below inherit from further up.
type Folder struct {
	ID          string
	Name        string
	Description string
	Auth        *Auth
	Scripts     *Scripts
	Items       []Item
}

// Item holds exactly one of Folder and Request.
type Item struct {
	Folder  *Folder
	Request *Request
}

// Request carries only the part its Protocol names; its Auth keeps none and inherit as they are.
type Request struct {
	ID          string
	Name        string
	Description string
	Protocol    entities.Protocol
	HTTP        *HTTPPart
	GraphQL     *GraphQLPart
	GRPC        *GRPCPart
	WebSocket   *WSPart
	Auth        *Auth
	Scripts     *Scripts
	Examples    []Example
}

type HTTPPart struct {
	Method  string
	URL     string
	Headers []Header
	Body    Body
}

type GraphQLPart struct {
	URL           string
	Headers       []Header
	Query         string
	Variables     string
	OperationName string
}

// GRPCPart.Metadata is the effective set: collection, then folders, then the request, the nearest
// level winning per lowercased key, sorted by key.
type GRPCPart struct {
	Target   string
	Service  string
	Method   string
	Message  string
	Metadata []Header
}

type WSPart struct {
	URL          string
	Headers      []Header
	Subprotocols []string
	Messages     []WSMessage
}

// WSMessage.Data is base64 for the binary format.
type WSMessage struct {
	Name   string
	Format string
	Data   string
}

// Header.Redacted marks a value whose literal parts were replaced with <redacted>.
type Header struct {
	Key      string
	Value    string
	Enabled  bool
	Redacted bool
}

// Body.FileName is the base name of a binary body; a file form field keeps its base name in Value.
type Body struct {
	Type     string
	Raw      string
	Fields   []FormField
	FileName string
}

type FormField struct {
	Key     string
	Value   string
	Type    string
	Enabled bool
}

// Auth.Redacted names the secret fields whose literal value was emptied.
type Auth struct {
	Type     string
	Fields   map[string]any
	Redacted []string
}

type Scripts struct {
	Pre  string
	Post string
}

type Example struct {
	ID          string
	Name        string
	Status      int
	StatusText  string
	Headers     []Header
	Body        string
	ContentType string
}

// Environment lists enabled variables only; a hidden one has Secret set and an empty Value.
type Environment struct {
	Name      string
	Variables []Variable
}

type Variable struct {
	Key    string
	Value  string
	Secret bool
}
