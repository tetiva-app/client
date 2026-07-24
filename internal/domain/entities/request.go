package entities

import (
	"time"

	"github.com/google/uuid"
)

// Protocol represents the communication protocol of a request.
type Protocol string

const (
	ProtocolHTTP      Protocol = "http"
	ProtocolGRPC      Protocol = "grpc"
	ProtocolGraphQL   Protocol = "graphql"
	ProtocolWebSocket Protocol = "websocket"
)

// IsValid checks whether the protocol value is supported.
func (p Protocol) IsValid() bool {
	switch p {
	case ProtocolHTTP, ProtocolGRPC, ProtocolGraphQL, ProtocolWebSocket:
		return true
	}
	return false
}

// ValidProtocols returns all supported protocol values.
func ValidProtocols() []Protocol {
	return []Protocol{ProtocolHTTP, ProtocolGRPC, ProtocolGraphQL, ProtocolWebSocket}
}

// HTTPMethod represents an HTTP request method.
type HTTPMethod string

const (
	MethodGET     HTTPMethod = "GET"
	MethodPOST    HTTPMethod = "POST"
	MethodPUT     HTTPMethod = "PUT"
	MethodPATCH   HTTPMethod = "PATCH"
	MethodDELETE  HTTPMethod = "DELETE"
	MethodOPTIONS HTTPMethod = "OPTIONS"
	MethodHEAD    HTTPMethod = "HEAD"
)

// IsValid checks whether the HTTP method value is supported.
func (m HTTPMethod) IsValid() bool {
	switch m {
	case MethodGET, MethodPOST, MethodPUT, MethodPATCH,
		MethodDELETE, MethodOPTIONS, MethodHEAD:
		return true
	}
	return false
}

// ValidHTTPMethods returns all supported HTTP method values.
func ValidHTTPMethods() []HTTPMethod {
	return []HTTPMethod{
		MethodGET, MethodPOST, MethodPUT, MethodPATCH,
		MethodDELETE, MethodOPTIONS, MethodHEAD,
	}
}

// BodyType represents the content type of the request body.
type BodyType string

const (
	BodyTypeNone   BodyType = "none"
	BodyTypeJSON   BodyType = "json"
	BodyTypeXML    BodyType = "xml"
	BodyTypeForm   BodyType = "form"
	BodyTypeBinary BodyType = "binary"
	BodyTypeRaw    BodyType = "raw"
)

// IsValid checks whether the body type value is supported.
func (b BodyType) IsValid() bool {
	switch b {
	case BodyTypeNone, BodyTypeJSON, BodyTypeXML, BodyTypeForm,
		BodyTypeBinary, BodyTypeRaw:
		return true
	}
	return false
}

// ValidBodyTypes returns all supported body type values.
func ValidBodyTypes() []BodyType {
	return []BodyType{
		BodyTypeNone, BodyTypeJSON, BodyTypeXML, BodyTypeForm,
		BodyTypeBinary, BodyTypeRaw,
	}
}

// AuthType represents the authentication method for a request.
type AuthType string

const (
	AuthTypeNone    AuthType = "none"
	AuthTypeBasic   AuthType = "basic"
	AuthTypeBearer  AuthType = "bearer"
	AuthTypeAPIKey  AuthType = "api_key"
	AuthTypeInherit AuthType = "inherit"
)

// IsValid checks whether the auth type value is supported.
func (a AuthType) IsValid() bool {
	switch a {
	case AuthTypeNone, AuthTypeBasic, AuthTypeBearer, AuthTypeAPIKey, AuthTypeInherit:
		return true
	}
	return false
}

// ValidAuthTypes returns all supported auth type values.
func ValidAuthTypes() []AuthType {
	return []AuthType{AuthTypeNone, AuthTypeBasic, AuthTypeBearer, AuthTypeAPIKey, AuthTypeInherit}
}

// HeaderItem represents a single header with enabled/disabled state.
type HeaderItem struct {
	Key     string
	Value   string
	Enabled bool
}

// EnabledHeadersToMap filters enabled headers and converts them to map[string][]string.
func EnabledHeadersToMap(items []HeaderItem) map[string][]string {
	result := make(map[string][]string)
	for _, item := range items {
		if item.Enabled && item.Key != "" {
			result[item.Key] = append(result[item.Key], item.Value)
		}
	}
	return result
}

// Request represents an HTTP or gRPC request within a collection.
type Request struct {
	ID                uuid.UUID
	CollectionID      uuid.UUID
	Name              string
	Protocol          Protocol
	Method            HTTPMethod
	URL               string
	Headers           []HeaderItem
	Body              string
	BodyType          BodyType
	AuthType          AuthType
	AuthData          string
	GRPCService       string
	GRPCMethod        string
	GRPCProtoPath     string
	GRPCMetadata      map[string][]string
	GraphQLQuery      string
	GraphQLVariables  string
	GraphQLSchemaPath string
	GraphQLOperation  string
	PreScript         string
	PostScript        string
	SortOrder         int
	Version           int
	IsDelete          bool
	// IsDraft marks a temporary request created via Replay from history;
	// drafts are excluded from List and hard-deleted when their tab closes.
	IsDraft   bool
	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
}
