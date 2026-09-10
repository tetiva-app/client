package entities

import (
	"time"

	"github.com/google/uuid"
)

type Protocol string

const (
	ProtocolHTTP      Protocol = "http"
	ProtocolGRPC      Protocol = "grpc"
	ProtocolGraphQL   Protocol = "graphql"
	ProtocolWebSocket Protocol = "websocket"
)

func (p Protocol) IsValid() bool {
	switch p {
	case ProtocolHTTP, ProtocolGRPC, ProtocolGraphQL, ProtocolWebSocket:
		return true
	}
	return false
}

func ValidProtocols() []Protocol {
	return []Protocol{ProtocolHTTP, ProtocolGRPC, ProtocolGraphQL, ProtocolWebSocket}
}

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

func (m HTTPMethod) IsValid() bool {
	switch m {
	case MethodGET, MethodPOST, MethodPUT, MethodPATCH,
		MethodDELETE, MethodOPTIONS, MethodHEAD:
		return true
	}
	return false
}

func ValidHTTPMethods() []HTTPMethod {
	return []HTTPMethod{
		MethodGET, MethodPOST, MethodPUT, MethodPATCH,
		MethodDELETE, MethodOPTIONS, MethodHEAD,
	}
}

type BodyType string

const (
	BodyTypeNone   BodyType = "none"
	BodyTypeJSON   BodyType = "json"
	BodyTypeXML    BodyType = "xml"
	BodyTypeForm   BodyType = "form"
	BodyTypeBinary BodyType = "binary"
	BodyTypeRaw    BodyType = "raw"
)

func (b BodyType) IsValid() bool {
	switch b {
	case BodyTypeNone, BodyTypeJSON, BodyTypeXML, BodyTypeForm,
		BodyTypeBinary, BodyTypeRaw:
		return true
	}
	return false
}

func ValidBodyTypes() []BodyType {
	return []BodyType{
		BodyTypeNone, BodyTypeJSON, BodyTypeXML, BodyTypeForm,
		BodyTypeBinary, BodyTypeRaw,
	}
}

type AuthType string

const (
	AuthTypeNone     AuthType = "none"
	AuthTypeBasic    AuthType = "basic"
	AuthTypeBearer   AuthType = "bearer"
	AuthTypeAPIKey   AuthType = "api_key"
	AuthTypeInherit  AuthType = "inherit"
	AuthTypeOAuth2   AuthType = "oauth2"
	AuthTypeJWT      AuthType = "jwt"
	AuthTypeDigest   AuthType = "digest"
	AuthTypeAWSSigV4 AuthType = "aws_sigv4"
)

func (a AuthType) IsValid() bool {
	for _, t := range ValidAuthTypes() {
		if a == t {
			return true
		}
	}
	return false
}

// ValidAuthTypes is the single registry: validation, MCP tool descriptions and the frontend fixture derive from it.
func ValidAuthTypes() []AuthType {
	return []AuthType{
		AuthTypeNone, AuthTypeBasic, AuthTypeBearer, AuthTypeAPIKey, AuthTypeInherit,
		AuthTypeOAuth2, AuthTypeJWT, AuthTypeDigest, AuthTypeAWSSigV4,
	}
}

// ValidCollectionAuthTypes drops inherit: a collection with auth_type none already delegates to its parent.
func ValidCollectionAuthTypes() []AuthType {
	all := ValidAuthTypes()
	types := make([]AuthType, 0, len(all)-1)
	for _, t := range all {
		if t != AuthTypeInherit {
			types = append(types, t)
		}
	}
	return types
}

type HeaderItem struct {
	Key     string
	Value   string
	Enabled bool
}

func EnabledHeadersToMap(items []HeaderItem) map[string][]string {
	result := make(map[string][]string)
	for _, item := range items {
		if item.Enabled && item.Key != "" {
			result[item.Key] = append(result[item.Key], item.Value)
		}
	}
	return result
}

type Request struct {
	ID                uuid.UUID
	CollectionID      uuid.UUID
	Name              string
	Description       string
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
