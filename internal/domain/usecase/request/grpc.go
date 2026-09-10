package request

import (
	"context"
	"fmt"
)

const grpcFuncPrefix = "request.usecase"

func (u *usecase) GRPCListServices(ctx context.Context, req GRPCConnectRequest) (*GRPCSchema, error) {
	return u.grpcRequester.ListServices(ctx, req)
}

func (u *usecase) GRPCGenerateExample(ctx context.Context, req GRPCConnectRequest, service, method string) (string, error) {
	schema, err := u.grpcRequester.ListServices(ctx, req)
	if err != nil {
		return "", fmt.Errorf("%s.GRPCGenerateExample: %w", grpcFuncPrefix, err)
	}

	for _, svc := range schema.Services {
		if svc.FullName == service {
			for _, m := range svc.Methods {
				if m.Name == method {
					return m.ExampleJSON, nil
				}
			}
		}
	}

	return "", fmt.Errorf("%s.GRPCGenerateExample: method %s/%s not found", grpcFuncPrefix, service, method)
}

func (u *usecase) GRPCGetProtoDefinition(ctx context.Context, req GRPCConnectRequest, service, method string) (string, error) {
	schema, err := u.grpcRequester.ListServices(ctx, req)
	if err != nil {
		return "", fmt.Errorf("%s.GRPCGetProtoDefinition: %w", grpcFuncPrefix, err)
	}

	for _, svc := range schema.Services {
		if svc.FullName == service {
			for _, m := range svc.Methods {
				if m.Name == method {
					return m.ProtoDefinition, nil
				}
			}
		}
	}

	return "", fmt.Errorf("%s.GRPCGetProtoDefinition: method %s/%s not found", grpcFuncPrefix, service, method)
}
