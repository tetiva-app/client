// Package publication builds the scrubbed, scanned snapshot a public page renders from.
package publication

import "github.com/tetiva-app/client/internal/domain/entities"

// Snapshot mirrors CollectionSnapshot v1; see adapters/publication/snapshotjson for JSON.
type Snapshot struct {
	Generator   string
	Locale      string
	Collection  Root
	Environment *Environment
}

type Root struct {
	Folder
	GRPCMetadata []Header
}

type Folder struct {
	ID          string
	Name        string
	Description string
	Auth        *Auth
	Scripts     *Scripts
	Items       []Item
}

type Item struct {
	Folder  *Folder
	Request *Request
}

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

type WSMessage struct {
	Name   string
	Format string
	Data   string
}

type Header struct {
	Key      string
	Value    string
	Enabled  bool
	Redacted bool
}

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

type Environment struct {
	Name      string
	Variables []Variable
}

type Variable struct {
	Key    string
	Value  string
	Secret bool
}
