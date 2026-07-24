package dto

import (
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type GRPCConnectRequest struct {
	Host      string `json:"host"`
	UseTLS    bool   `json:"useTls"`
	ProtoPath string `json:"protoPath"`
}

type GRPCGenerateExampleRequest struct {
	Host      string `json:"host"`
	UseTLS    bool   `json:"useTls"`
	ProtoPath string `json:"protoPath"`
	Service   string `json:"service"`
	Method    string `json:"method"`
}

type GRPCGetProtoDefinitionRequest struct {
	Host      string `json:"host"`
	UseTLS    bool   `json:"useTls"`
	ProtoPath string `json:"protoPath"`
	Service   string `json:"service"`
	Method    string `json:"method"`
}

type GRPCSchemaResponse struct {
	Services []GRPCServiceResponse `json:"services"`
	Source   string                `json:"source"`
}

type GRPCServiceResponse struct {
	FullName string                   `json:"fullName"`
	Methods  []GRPCMethodInfoResponse `json:"methods"`
}

type GRPCMethodInfoResponse struct {
	Name            string `json:"name"`
	InputType       string `json:"inputType"`
	OutputType      string `json:"outputType"`
	IsServerStream  bool   `json:"isServerStream"`
	IsClientStream  bool   `json:"isClientStream"`
	ProtoDefinition string `json:"protoDefinition"`
	ExampleJSON     string `json:"exampleJson"`
}

// GRPCSchemaToResponse converts domain GRPCSchema to DTO.
func GRPCSchemaToResponse(schema *request.GRPCSchema) GRPCSchemaResponse {
	resp := GRPCSchemaResponse{Source: schema.Source}
	for _, svc := range schema.Services {
		svcResp := GRPCServiceResponse{FullName: svc.FullName}
		for _, m := range svc.Methods {
			svcResp.Methods = append(svcResp.Methods, GRPCMethodInfoResponse{
				Name:            m.Name,
				InputType:       m.InputType,
				OutputType:      m.OutputType,
				IsServerStream:  m.IsServerStream,
				IsClientStream:  m.IsClientStream,
				ProtoDefinition: m.ProtoDefinition,
				ExampleJSON:     m.ExampleJSON,
			})
		}
		resp.Services = append(resp.Services, svcResp)
	}
	return resp
}
