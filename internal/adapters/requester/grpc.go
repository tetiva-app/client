package requester

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// jhump/protoreflect packages below are deprecated in favour of
	// google.golang.org/protobuf v2 + bufbuild/protocompile, but
	// dynamic/grpcdynamic/grpcreflect have no drop-in v2 replacements yet,
	// so the migration is deferred.
	"github.com/jhump/protoreflect/desc"            //nolint:staticcheck // SA1019: deferred migration
	"github.com/jhump/protoreflect/desc/protoparse" //nolint:staticcheck // SA1019: deferred migration
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/jhump/protoreflect/dynamic" //nolint:staticcheck // SA1019: no v2 equivalent yet
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpcmd "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const (
	grpcConnectTimeout = 30 * time.Second
	grpcRequestTimeout = 30 * time.Second
)

// GRPCRequester implements request.GRPCRequester using protoreflect and grpc.
type GRPCRequester struct{}

// NewGRPCRequester creates a new GRPCRequester.
func NewGRPCRequester() *GRPCRequester {
	return &GRPCRequester{}
}

// Execute sends a gRPC unary request and returns the response.
func (r *GRPCRequester) Execute(ctx context.Context, req request.GRPCExecuteRequest) (*entities.Response, error) {
	const funcName = "GRPCRequester.Execute"

	conn, err := connectToServer(req.Host, req.UseTLS)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = conn.Close() }()

	methodDesc, err := resolveMethod(ctx, conn, req.ProtoPath, req.Service, req.Method)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	inputMsg := dynamic.NewMessage(methodDesc.GetInputType())
	if req.Message != "" {
		if err := inputMsg.UnmarshalJSON([]byte(req.Message)); err != nil {
			return nil, fmt.Errorf("%s: failed to parse request message JSON: %w", funcName, err)
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, grpcRequestTimeout)
	defer cancel()

	if len(req.Metadata) > 0 {
		md := grpcmd.MD(map[string][]string(req.Metadata))
		callCtx = grpcmd.NewOutgoingContext(callCtx, md)
	}

	var respHeaders, respTrailers grpcmd.MD
	callOpts := []grpc.CallOption{
		grpc.Header(&respHeaders),
		grpc.Trailer(&respTrailers),
	}

	stub := grpcdynamic.NewStub(conn)
	start := time.Now()
	resp, err := stub.InvokeRpc(callCtx, methodDesc, inputMsg, callOpts...)
	duration := time.Since(start)

	if err != nil {
		// Even on error, return a Response with gRPC status info.
		st, _ := status.FromError(err)
		headers := mergeMetadata(respHeaders, respTrailers)

		return &entities.Response{
			StatusCode: int(st.Code()),
			StatusText: st.Code().String(),
			Body:       st.Message(),
			Headers:    headers,
			Size:       int64(len(st.Message())),
			Duration:   duration,
			Protocol:   entities.ProtocolGRPC,
		}, nil
	}

	var respBody string
	if dynMsg, ok := resp.(*dynamic.Message); ok {
		bodyBytes, marshalErr := dynMsg.MarshalJSONIndent()
		if marshalErr != nil {
			return nil, fmt.Errorf("%s: failed to marshal response JSON: %w", funcName, marshalErr)
		}
		respBody = string(bodyBytes)
	} else {
		bodyBytes, marshalErr := json.MarshalIndent(resp, "", "  ")
		if marshalErr != nil {
			return nil, fmt.Errorf("%s: failed to marshal response JSON: %w", funcName, marshalErr)
		}
		respBody = string(bodyBytes)
	}

	headers := mergeMetadata(respHeaders, respTrailers)

	return &entities.Response{
		StatusCode: int(codes.OK),
		StatusText: codes.OK.String(),
		Headers:    headers,
		Body:       respBody,
		Size:       int64(len(respBody)),
		Duration:   duration,
		Protocol:   entities.ProtocolGRPC,
	}, nil
}

// ListServices connects to a gRPC server and returns available services and methods.
func (r *GRPCRequester) ListServices(ctx context.Context, req request.GRPCConnectRequest) (*request.GRPCSchema, error) {
	if req.ProtoPath != "" {
		return r.listServicesFromProto(req.ProtoPath)
	}

	return r.listServicesFromReflection(ctx, req.Host, req.UseTLS)
}

func (r *GRPCRequester) listServicesFromReflection(ctx context.Context, host string, useTLS bool) (*request.GRPCSchema, error) {
	const funcName = "GRPCRequester.listServicesFromReflection"

	conn, err := connectToServer(host, useTLS)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = conn.Close() }()

	refClient := grpcreflect.NewClientAuto(ctx, conn)
	defer refClient.Reset()

	serviceNames, err := refClient.ListServices()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to list services: %w", funcName, err)
	}

	var services []request.GRPCService
	for _, svcName := range serviceNames {
		if strings.HasPrefix(svcName, "grpc.reflection.") {
			continue
		}

		svcDesc, err := refClient.ResolveService(svcName)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to resolve service %q: %w", funcName, svcName, err)
		}

		svc := buildGRPCService(svcDesc)
		services = append(services, svc)
	}

	return &request.GRPCSchema{
		Services: services,
		Source:   "reflection",
	}, nil
}

func (r *GRPCRequester) listServicesFromProto(protoPath string) (*request.GRPCSchema, error) {
	const funcName = "GRPCRequester.listServicesFromProto"

	info, err := os.Stat(protoPath)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to access proto path %q: %w", funcName, protoPath, err)
	}

	var fileDescs []*desc.FileDescriptor
	var source string

	if info.IsDir() {
		fileDescs, err = parseProtoDirectory(protoPath)
		source = "proto_directory"
	} else {
		fileDescs, err = parseProtoFile(protoPath)
		source = "proto_file"
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	var services []request.GRPCService
	for _, fd := range fileDescs {
		for _, svcDesc := range fd.GetServices() {
			svc := buildGRPCService(svcDesc)
			services = append(services, svc)
		}
	}

	return &request.GRPCSchema{
		Services: services,
		Source:   source,
	}, nil
}

func connectToServer(host string, useTLS bool) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), grpcConnectTimeout)
	defer cancel()

	var opts []grpc.DialOption
	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// grpc.NewClient connects lazily, which changes connect-error timing;
	// deferred with the protoreflect migration.
	conn, err := grpc.DialContext(ctx, host, opts...) //nolint:staticcheck // SA1019: deferred migration
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %q: %w", host, err)
	}

	return conn, nil
}

func resolveMethod(ctx context.Context, conn *grpc.ClientConn, protoPath, service, method string) (*desc.MethodDescriptor, error) {
	var svcDesc *desc.ServiceDescriptor

	if protoPath != "" {
		info, err := os.Stat(protoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to access proto path %q: %w", protoPath, err)
		}

		var fileDescs []*desc.FileDescriptor
		if info.IsDir() {
			fileDescs, err = parseProtoDirectory(protoPath)
		} else {
			fileDescs, err = parseProtoFile(protoPath)
		}
		if err != nil {
			return nil, err
		}

		for _, fd := range fileDescs {
			if sd := fd.FindService(service); sd != nil {
				svcDesc = sd
				break
			}
		}
	} else {
		refClient := grpcreflect.NewClientAuto(ctx, conn)
		defer refClient.Reset()

		var err error
		svcDesc, err = refClient.ResolveService(service)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve service %q via reflection: %w", service, err)
		}
	}

	if svcDesc == nil {
		return nil, fmt.Errorf("service %q not found", service)
	}

	methodDesc := svcDesc.FindMethodByName(method)
	if methodDesc == nil {
		return nil, fmt.Errorf("method %q not found in service %q", method, service)
	}

	return methodDesc, nil
}

func parseProtoFile(filePath string) ([]*desc.FileDescriptor, error) {
	dir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	// Walk up parent directories for cross-directory imports
	importPaths := []string{dir}
	d := dir
	for i := 0; i < 5; i++ {
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		importPaths = append(importPaths, parent)
		d = parent
	}

	parser := protoparse.Parser{
		ImportPaths:           importPaths,
		InferImportPaths:      true,
		IncludeSourceCodeInfo: true,
	}

	fds, err := parser.ParseFiles(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto file %q: %w", filePath, err)
	}

	return fds, nil
}

func parseProtoDirectory(dirPath string) ([]*desc.FileDescriptor, error) {
	var protoFiles []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			relPath, relErr := filepath.Rel(dirPath, path)
			if relErr != nil {
				return relErr
			}
			protoFiles = append(protoFiles, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk proto directory %q: %w", dirPath, err)
	}

	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("no .proto files found in directory %q", dirPath)
	}

	// Proto imports like "task/v1/issue.proto" resolve relative to a root, but
	// the user may have selected a subdirectory — add ancestors as import paths.
	importPaths := []string{dirPath}
	dir := dirPath
	for i := 0; i < 5; i++ {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		importPaths = append(importPaths, parent)
		dir = parent
	}

	parser := protoparse.Parser{
		ImportPaths:           importPaths,
		InferImportPaths:      true,
		IncludeSourceCodeInfo: true,
	}

	fds, err := parser.ParseFiles(protoFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto files in %q: %w", dirPath, err)
	}

	return fds, nil
}

func buildGRPCService(svcDesc *desc.ServiceDescriptor) request.GRPCService {
	var methods []request.GRPCMethodInfo

	for _, md := range svcDesc.GetMethods() {
		methods = append(methods, request.GRPCMethodInfo{
			Name:            md.GetName(),
			InputType:       md.GetInputType().GetFullyQualifiedName(),
			OutputType:      md.GetOutputType().GetFullyQualifiedName(),
			IsServerStream:  md.IsServerStreaming(),
			IsClientStream:  md.IsClientStreaming(),
			ProtoDefinition: protoDefinitionText(svcDesc, md),
			ExampleJSON:     generateExampleJSON(md.GetInputType()),
		})
	}

	return request.GRPCService{
		FullName: svcDesc.GetFullyQualifiedName(),
		Methods:  methods,
	}
}

func generateExampleJSON(md *desc.MessageDescriptor) string {
	visited := make(map[string]bool)
	result := generateExampleObject(md, visited)

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "{}"
	}

	return string(jsonBytes)
}

func generateExampleObject(md *desc.MessageDescriptor, visited map[string]bool) map[string]any {
	fqn := md.GetFullyQualifiedName()

	// Prevent infinite recursion on self-referencing messages.
	if visited[fqn] {
		return map[string]any{}
	}
	visited[fqn] = true
	defer func() { visited[fqn] = false }()

	result := make(map[string]any)

	for _, fd := range md.GetFields() {
		name := fd.GetJSONName()
		if name == "" {
			name = fd.GetName()
		}

		if fd.IsMap() {
			keyType := fd.GetMapKeyType()
			valType := fd.GetMapValueType()
			exampleKey := zeroValueForField(keyType, visited)
			exampleVal := zeroValueForField(valType, visited)
			result[name] = map[string]any{
				fmt.Sprintf("%v", exampleKey): exampleVal,
			}
			continue
		}

		val := zeroValueForField(fd, visited)
		if fd.IsRepeated() {
			result[name] = []any{val}
		} else {
			result[name] = val
		}
	}

	return result
}

func zeroValueForField(fd *desc.FieldDescriptor, visited map[string]bool) any {
	switch fd.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
		descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return 0.0

	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return 0

	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return false

	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return ""

	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return ""

	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		enumDesc := fd.GetEnumType()
		if enumDesc != nil {
			values := enumDesc.GetValues()
			if len(values) > 0 {
				return values[0].GetName()
			}
		}
		return 0

	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		msgDesc := fd.GetMessageType()
		if msgDesc != nil {
			return generateExampleObject(msgDesc, visited)
		}
		return map[string]any{}

	default:
		return nil
	}
}

func protoDefinitionText(sd *desc.ServiceDescriptor, md *desc.MethodDescriptor) string {
	printer := protoprint.Printer{
		Compact: true,
	}

	var buf bytes.Buffer

	inputDef, err := printer.PrintProtoToString(md.GetInputType())
	if err == nil {
		buf.WriteString(strings.TrimSpace(inputDef))
		buf.WriteString("\n\n")
	}

	outputDef, err := printer.PrintProtoToString(md.GetOutputType())
	if err == nil {
		buf.WriteString(strings.TrimSpace(outputDef))
		buf.WriteString("\n\n")
	}

	fmt.Fprintf(&buf, "service %s {\n", sd.GetName())

	clientStream := ""
	serverStream := ""
	if md.IsClientStreaming() {
		clientStream = "stream "
	}
	if md.IsServerStreaming() {
		serverStream = "stream "
	}

	fmt.Fprintf(&buf, "  rpc %s (%s%s) returns (%s%s);\n",
		md.GetName(),
		clientStream, md.GetInputType().GetName(),
		serverStream, md.GetOutputType().GetName(),
	)
	buf.WriteString("}")

	return buf.String()
}

func mergeMetadata(headers, trailers grpcmd.MD) map[string][]string {
	result := make(map[string][]string)

	for k, v := range headers {
		result[k] = append(result[k], v...)
	}
	for k, v := range trailers {
		result[k] = append(result[k], v...)
	}

	return result
}
