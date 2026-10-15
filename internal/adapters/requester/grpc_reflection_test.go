package requester

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	refv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestGRPCRequester_ListServicesSurvivesFrozenConnection(t *testing.T) {
	shortGRPCTimeouts(t, 5*time.Second, time.Second)
	files := serviceFiles(t, map[string]int{"big": 600, "small": 1})
	lis := freezingListener{Listener: listenTCP(t), limit: 8 << 10}
	addr := startReflectionServer(t, lis, files, []string{"big.Service", "small.Service"},
		grpc.StreamInterceptor(delayLookup("small.Service", 200*time.Millisecond)))

	var schema *request.GRPCSchema
	var err error
	returnsWithin(t, 3*time.Second, func() {
		schema, err = NewGRPCRequester().ListServices(context.Background(), request.GRPCConnectRequest{Host: addr})
	})
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(schema.Services) != 1 || schema.Services[0].FullName != "small.Service" {
		t.Fatalf("services = %+v", schema.Services)
	}
	if len(schema.Warnings) != 1 || schema.Warnings[0] != "big.Service: timed out" {
		t.Fatalf("warnings = %q", schema.Warnings)
	}
}

func TestGRPCRequester_ListServicesSkipsUnresolvableService(t *testing.T) {
	files := serviceFiles(t, map[string]int{"small": 1})
	addr := startReflectionServer(t, listenTCP(t), files, []string{"ghost.Service", "small.Service"})

	schema, err := NewGRPCRequester().ListServices(context.Background(), request.GRPCConnectRequest{Host: addr})
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(schema.Services) != 1 || schema.Services[0].FullName != "small.Service" {
		t.Fatalf("services = %+v", schema.Services)
	}
	if len(schema.Warnings) != 1 || !strings.HasPrefix(schema.Warnings[0], "ghost.Service: ") {
		t.Fatalf("warnings = %q", schema.Warnings)
	}
}

func TestGRPCRequester_ListServicesFailsWhenNothingResolves(t *testing.T) {
	addr := startReflectionServer(t, listenTCP(t), new(protoregistry.Files), []string{"ghost.Service"})

	_, err := NewGRPCRequester().ListServices(context.Background(), request.GRPCConnectRequest{Host: addr})
	if err == nil || !strings.Contains(err.Error(), "ghost.Service") {
		t.Fatalf("err = %v", err)
	}
}

func TestGRPCRequester_DescribeService(t *testing.T) {
	files := serviceFiles(t, map[string]int{"small": 2})
	addr := startReflectionServer(t, listenTCP(t), files, []string{"ghost.Service", "small.Service"})
	r := NewGRPCRequester()
	ctx := context.Background()

	svc, err := r.DescribeService(ctx, request.GRPCConnectRequest{Host: addr}, "small.Service")
	if err != nil {
		t.Fatalf("reflection: %v", err)
	}
	if svc.FullName != "small.Service" || len(svc.Methods) != 1 || !strings.Contains(svc.Methods[0].ExampleJSON, "fieldWithAReasonablyLongName2") {
		t.Fatalf("reflection: %+v", svc)
	}

	svc, err = r.DescribeService(ctx, request.GRPCConnectRequest{ProtoPath: healthProto(t)}, "grpc.health.v1.Health")
	if err != nil || svc.FullName != "grpc.health.v1.Health" || svc.Methods[0].Name != "Check" {
		t.Fatalf("proto file: %+v, %v", svc, err)
	}

	if _, err := r.DescribeService(ctx, request.GRPCConnectRequest{Host: addr}, "ghost.Service"); err == nil {
		t.Fatal("ghost.Service resolved")
	}
}

func TestGRPCRequester_SilentReflectionTimesOut(t *testing.T) {
	shortGRPCTimeouts(t, 300*time.Millisecond, 200*time.Millisecond)
	gs := grpc.NewServer()
	refv1.RegisterServerReflectionServer(gs, silentReflection{})
	lis := listenTCP(t)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	addr := lis.Addr().String()
	r := NewGRPCRequester()

	returnsWithin(t, 3*time.Second, func() {
		_, err := r.ListServices(context.Background(), request.GRPCConnectRequest{Host: addr})
		if err == nil || !strings.Contains(err.Error(), "DeadlineExceeded") {
			t.Errorf("ListServices err = %v", err)
		}
	})
	returnsWithin(t, 3*time.Second, func() {
		_, err := r.DescribeService(context.Background(), request.GRPCConnectRequest{Host: addr}, "a.Service")
		if err == nil || !strings.Contains(err.Error(), "DeadlineExceeded") {
			t.Errorf("DescribeService err = %v", err)
		}
	})
	returnsWithin(t, 3*time.Second, func() {
		_, err := r.Execute(context.Background(), request.GRPCExecuteRequest{Host: addr, Service: "a.Service", Method: "Call"})
		if err == nil || !strings.Contains(err.Error(), "DeadlineExceeded") {
			t.Errorf("Execute err = %v", err)
		}
	})
}

type silentReflection struct {
	refv1.UnimplementedServerReflectionServer
}

func (silentReflection) ServerReflectionInfo(stream refv1.ServerReflection_ServerReflectionInfoServer) error {
	<-stream.Context().Done()
	return nil
}

type freezingListener struct {
	net.Listener
	limit int
}

func (l freezingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &freezingConn{Conn: c, limit: l.limit}, nil
}

type freezingConn struct {
	net.Conn
	mu    sync.Mutex
	limit int
	sent  int
}

func (c *freezingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sent+len(p) > c.limit {
		c.sent = c.limit
		return len(p), nil
	}
	c.sent += len(p)
	return c.Conn.Write(p)
}

func delayLookup(symbol string, delay time.Duration) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, delayedStream{ServerStream: ss, symbol: symbol, delay: delay})
	}
}

type delayedStream struct {
	grpc.ServerStream
	symbol string
	delay  time.Duration
}

func (s delayedStream) RecvMsg(m any) error {
	err := s.ServerStream.RecvMsg(m)
	if req, ok := m.(*refv1.ServerReflectionRequest); ok && req.GetFileContainingSymbol() == s.symbol {
		time.Sleep(s.delay)
	}
	return err
}

type serviceSet map[string]grpc.ServiceInfo

func (s serviceSet) GetServiceInfo() map[string]grpc.ServiceInfo { return s }

func startReflectionServer(t *testing.T, lis net.Listener, files *protoregistry.Files, services []string, opts ...grpc.ServerOption) string {
	t.Helper()
	set := serviceSet{}
	for _, s := range services {
		set[s] = grpc.ServiceInfo{}
	}
	gs := grpc.NewServer(opts...)
	refv1.RegisterServerReflectionServer(gs, reflection.NewServerV1(reflection.ServerOptions{Services: set, DescriptorResolver: files}))
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return lis.Addr().String()
}

func serviceFiles(t *testing.T, fieldsByPackage map[string]int) *protoregistry.Files {
	t.Helper()
	files := new(protoregistry.Files)
	for pkg, fields := range fieldsByPackage {
		msg := &descriptorpb.DescriptorProto{Name: proto.String("Payload")}
		for i := 1; i <= fields; i++ {
			msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
				Name:   proto.String(fmt.Sprintf("field_with_a_reasonably_long_name_%d", i)),
				Number: proto.Int32(int32(i)),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			})
		}
		fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
			Name:        proto.String(pkg + ".proto"),
			Package:     proto.String(pkg),
			Syntax:      proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{msg},
			Service: []*descriptorpb.ServiceDescriptorProto{{
				Name: proto.String("Service"),
				Method: []*descriptorpb.MethodDescriptorProto{{
					Name:       proto.String("Call"),
					InputType:  proto.String("." + pkg + ".Payload"),
					OutputType: proto.String("." + pkg + ".Payload"),
				}},
			}},
		}, files)
		if err != nil {
			t.Fatal(err)
		}
		if err := files.RegisterFile(fd); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func listenTCP(t *testing.T) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return lis
}

func shortGRPCTimeouts(t *testing.T, requestTimeout, serviceTimeout time.Duration) {
	t.Helper()
	prevRequest, prevService := grpcRequestTimeout, grpcServiceTimeout
	grpcRequestTimeout, grpcServiceTimeout = requestTimeout, serviceTimeout
	t.Cleanup(func() { grpcRequestTimeout, grpcServiceTimeout = prevRequest, prevService })
}

func returnsWithin(t *testing.T, limit time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Fatalf("did not return within %s", limit)
	}
}
