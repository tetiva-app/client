package dto

import (
	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/har"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type HARNameValueDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HARParamDTO struct {
	Name        string `json:"name"`
	Value       string `json:"value"` // no omitempty: empty values are real
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

type HARPostDataDTO struct {
	MimeType string        `json:"mimeType"`
	Text     string        `json:"text"`
	Params   []HARParamDTO `json:"params"`
}

type HARTetivaDTO struct {
	BinaryFile string `json:"binaryFile,omitempty"`
	AuthNote   string `json:"authNote,omitempty"`
}

// HARRequestDTO keeps the HAR 1.2 field names: the snippet bundle hands it to httpsnippet as is.
type HARRequestDTO struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	HTTPVersion string            `json:"httpVersion"`
	Headers     []HARNameValueDTO `json:"headers"`
	QueryString []HARNameValueDTO `json:"queryString"`
	Cookies     []HARNameValueDTO `json:"cookies"`
	PostData    *HARPostDataDTO   `json:"postData,omitempty"`
	HeadersSize int               `json:"headersSize"`
	BodySize    int               `json:"bodySize"`
	Tetiva      *HARTetivaDTO     `json:"_tetiva,omitempty"`
}

type GRPCSnippetDTO struct {
	Target   string              `json:"target"`
	Service  string              `json:"service"`
	Method   string              `json:"method"`
	Message  string              `json:"message"`
	Metadata map[string][]string `json:"metadata"`
}

type WSSnippetMessageDTO struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Data   string `json:"data"`
}

type WSSnippetDTO struct {
	URL          string                `json:"url"`
	Headers      map[string][]string   `json:"headers"`
	Subprotocols []string              `json:"subprotocols"`
	Messages     []WSSnippetMessageDTO `json:"messages"`
}

type SnippetInputDTO struct {
	Protocol string          `json:"protocol"`
	HAR      *HARRequestDTO  `json:"har,omitempty"`
	GRPC     *GRPCSnippetDTO `json:"grpc,omitempty"`
	WS       *WSSnippetDTO   `json:"ws,omitempty"`
	Warnings []string        `json:"warnings"`
}

// SnippetRequestDTO is the unsaved editor state; field names match RequestResponse.
type SnippetRequestDTO struct {
	ID               string              `json:"id"`
	CollectionID     string              `json:"collectionId"`
	Protocol         string              `json:"protocol"`
	Method           string              `json:"method"`
	URL              string              `json:"url"`
	Headers          []HeaderItemDTO     `json:"headers"`
	Body             string              `json:"body"`
	BodyType         string              `json:"bodyType"`
	AuthType         string              `json:"authType"`
	AuthData         string              `json:"authData"`
	PreScript        string              `json:"preScript"`
	GRPCService      string              `json:"grpcService"`
	GRPCMethod       string              `json:"grpcMethod"`
	GRPCMetadata     map[string][]string `json:"grpcMetadata"`
	GraphQLQuery     string              `json:"graphqlQuery"`
	GraphQLVariables string              `json:"graphqlVariables"`
	GraphQLOperation string              `json:"graphqlOperation"`
}

type BuildSnippetRequest struct {
	WorkspaceID      string            `json:"workspaceId"`
	ResolveVariables bool              `json:"resolveVariables"`
	IncludeSecrets   bool              `json:"includeSecrets"`
	Request          SnippetRequestDTO `json:"request"`
}

func (d SnippetRequestDTO) ToEntity() (*entities.Request, error) {
	fields := map[string]string{}
	id, err := uuid.Parse(d.ID)
	if err != nil {
		fields["id"] = "invalid UUID"
	}
	collectionID, err := uuid.Parse(d.CollectionID)
	if err != nil {
		fields["collectionId"] = "invalid UUID"
	}
	if len(fields) > 0 {
		return nil, &domain.ValidationError{Fields: fields}
	}

	return &entities.Request{
		ID:               id,
		CollectionID:     collectionID,
		Protocol:         entities.Protocol(d.Protocol),
		Method:           entities.HTTPMethod(d.Method),
		URL:              d.URL,
		Headers:          HeaderItemsToEntity(d.Headers),
		Body:             d.Body,
		BodyType:         entities.BodyType(d.BodyType),
		AuthType:         entities.AuthType(d.AuthType),
		AuthData:         d.AuthData,
		PreScript:        d.PreScript,
		GRPCService:      d.GRPCService,
		GRPCMethod:       d.GRPCMethod,
		GRPCMetadata:     d.GRPCMetadata,
		GraphQLQuery:     d.GraphQLQuery,
		GraphQLVariables: d.GraphQLVariables,
		GraphQLOperation: d.GraphQLOperation,
	}, nil
}

// SnippetInputToDTO never emits null for a slice or map: the generators iterate them unguarded.
func SnippetInputToDTO(in request.SnippetInput) SnippetInputDTO {
	out := SnippetInputDTO{
		Protocol: string(in.Protocol),
		Warnings: nonNilStrings(in.Warnings),
	}
	if in.HAR != nil {
		h := harRequestToDTO(in.HAR)
		out.HAR = &h
	}
	if in.GRPC != nil {
		out.GRPC = &GRPCSnippetDTO{
			Target:   in.GRPC.Target,
			Service:  in.GRPC.Service,
			Method:   in.GRPC.Method,
			Message:  in.GRPC.Message,
			Metadata: nonNilHeaderMap(in.GRPC.Metadata),
		}
	}
	if in.WS != nil {
		messages := make([]WSSnippetMessageDTO, 0, len(in.WS.Messages))
		for _, m := range in.WS.Messages {
			messages = append(messages, WSSnippetMessageDTO{Name: m.Name, Format: m.Format, Data: m.Data})
		}
		out.WS = &WSSnippetDTO{
			URL:          in.WS.URL,
			Headers:      nonNilHeaderMap(in.WS.Headers),
			Subprotocols: nonNilStrings(in.WS.Subprotocols),
			Messages:     messages,
		}
	}
	return out
}

func harRequestToDTO(r *har.Request) HARRequestDTO {
	out := HARRequestDTO{
		Method:      r.Method,
		URL:         r.URL,
		HTTPVersion: r.HTTPVersion,
		Headers:     nameValuesToDTO(r.Headers),
		QueryString: nameValuesToDTO(r.QueryString),
		Cookies:     []HARNameValueDTO{},
		HeadersSize: -1,
		BodySize:    -1,
	}
	if r.PostData != nil {
		params := make([]HARParamDTO, 0, len(r.PostData.Params))
		for _, p := range r.PostData.Params {
			params = append(params, HARParamDTO{Name: p.Name, Value: p.Value, FileName: p.FileName, ContentType: p.ContentType})
		}
		out.PostData = &HARPostDataDTO{MimeType: r.PostData.MimeType, Text: r.PostData.Text, Params: params}
	}
	if r.BinaryFile != "" || r.AuthNote != "" {
		out.Tetiva = &HARTetivaDTO{BinaryFile: r.BinaryFile, AuthNote: r.AuthNote}
	}
	return out
}

func nameValuesToDTO(nvs []har.NameValue) []HARNameValueDTO {
	out := make([]HARNameValueDTO, 0, len(nvs))
	for _, nv := range nvs {
		out = append(out, HARNameValueDTO{Name: nv.Name, Value: nv.Value})
	}
	return out
}

func nonNilHeaderMap(h map[string][]string) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, values := range h {
		out[k] = nonNilStrings(values)
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
