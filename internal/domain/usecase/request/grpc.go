package request

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const grpcFuncPrefix = "request.usecase"

func (u *usecase) GRPCListServices(ctx context.Context, req GRPCConnectRequest) (*GRPCSchema, error) {
	req, err := u.resolveGRPCHost(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s.GRPCListServices: %w", grpcFuncPrefix, err)
	}
	return u.grpcRequester.ListServices(ctx, req)
}

func (u *usecase) GRPCGenerateExample(ctx context.Context, req GRPCConnectRequest, service, method string) (string, error) {
	m, err := u.grpcMethod(ctx, req, service, method)
	if err != nil {
		return "", fmt.Errorf("%s.GRPCGenerateExample: %w", grpcFuncPrefix, err)
	}
	return m.ExampleJSON, nil
}

func (u *usecase) GRPCGetProtoDefinition(ctx context.Context, req GRPCConnectRequest, service, method string) (string, error) {
	m, err := u.grpcMethod(ctx, req, service, method)
	if err != nil {
		return "", fmt.Errorf("%s.GRPCGetProtoDefinition: %w", grpcFuncPrefix, err)
	}
	return m.ProtoDefinition, nil
}

func (u *usecase) grpcMethod(ctx context.Context, req GRPCConnectRequest, service, method string) (*GRPCMethodInfo, error) {
	req, err := u.resolveGRPCHost(ctx, req)
	if err != nil {
		return nil, err
	}
	svc, err := u.grpcRequester.DescribeService(ctx, req, service)
	if err != nil {
		return nil, err
	}
	for i := range svc.Methods {
		if svc.Methods[i].Name == method {
			return &svc.Methods[i], nil
		}
	}
	return nil, fmt.Errorf("method %s/%s not found", service, method)
}

func (u *usecase) resolveGRPCHost(ctx context.Context, req GRPCConnectRequest) (GRPCConnectRequest, error) {
	vars, err := u.schemaVariables(ctx, req.WorkspaceID)
	if err != nil {
		return req, err
	}
	req.Host = substituteVariables(req.Host, vars)
	return req, nil
}

func (u *usecase) schemaVariables(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	if workspaceID == uuid.Nil {
		return nil, nil
	}
	return u.envResolver.ResolveVariables(ctx, workspaceID)
}
