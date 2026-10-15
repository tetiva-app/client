package requester

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestGRPCTarget(t *testing.T) {
	tests := []struct {
		address string
		target  string
		useTLS  bool
	}{
		{"localhost:50051", "localhost:50051", false},
		{"grpc://localhost:50051", "localhost:50051", false},
		{"grpcs://api.example.com:8443", "api.example.com:8443", true},
		{"grpcs://api.example.com", "api.example.com:443", true},
		{"GRPCS://api.example.com", "api.example.com:443", true},
		{" grpcs://api.example.com/ ", "api.example.com:443", true},
		{"grpcs://[::1]", "[::1]:443", true},
		{"grpcs://[::1]:9443", "[::1]:9443", true},
		{"dns:///api.example.com:443", "dns:///api.example.com:443", false},
		{"https://api.example.com", "api.example.com:443", true},
		{"HTTPS://api.example.com:8443/", "api.example.com:8443", true},
		{"http://localhost:50051", "localhost:50051", false},
	}
	for _, tt := range tests {
		target, useTLS := grpcTarget(tt.address)
		if target != tt.target || useTLS != tt.useTLS {
			t.Errorf("grpcTarget(%q) = %q, %v; want %q, %v", tt.address, target, useTLS, tt.target, tt.useTLS)
		}
	}
}

func TestTLSHintNamesTheTypedScheme(t *testing.T) {
	const notTLS = "first record does not look like a TLS handshake"
	const wantsTLS = "error reading server preface"
	tests := []struct{ address, msg, want string }{
		{"grpcs://h:1", notTLS, "The server may not use TLS — remove grpcs://"},
		{"HTTPS://h:1", notTLS, "The server may not use TLS — remove https://"},
		{"h:1", wantsTLS, "The server may require TLS — use grpcs://"},
		{"grpc://h:1", wantsTLS, "The server may require TLS — use grpcs://"},
		{"http://h:1", wantsTLS, "The server may require TLS — use https://"},
		{"https://h:1", wantsTLS, ""},
		{"h:1", notTLS, ""},
	}
	for _, tt := range tests {
		if got := tlsHint(tt.msg, tt.address); got != tt.want {
			t.Errorf("tlsHint(%q, %q) = %q, want %q", tt.msg, tt.address, got, tt.want)
		}
	}
}

func TestGRPCRequester_TLS(t *testing.T) {
	cert, roots := selfSignedCert(t)
	tlsAddr := startHealthServer(t, grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	plainAddr := startHealthServer(t)
	r := &GRPCRequester{rootCAs: roots}
	ctx := context.Background()

	t.Run("reflection and invoke over grpcs", func(t *testing.T) {
		schema, err := r.ListServices(ctx, request.GRPCConnectRequest{Host: "grpcs://" + tlsAddr})
		if err != nil {
			t.Fatalf("ListServices: %v", err)
		}
		if len(schema.Services) != 1 || schema.Services[0].FullName != "grpc.health.v1.Health" {
			t.Fatalf("services = %+v", schema.Services)
		}

		resp, err := r.Execute(ctx, request.GRPCExecuteRequest{
			Host:    "grpcs://" + tlsAddr,
			Service: "grpc.health.v1.Health",
			Method:  "Check",
			Message: "{}",
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if resp.StatusCode != 0 || !strings.Contains(resp.Body, "SERVING") {
			t.Fatalf("status %d body %q", resp.StatusCode, resp.Body)
		}
	})

	t.Run("plaintext stays the default", func(t *testing.T) {
		for _, host := range []string{plainAddr, "grpc://" + plainAddr} {
			if _, err := r.ListServices(ctx, request.GRPCConnectRequest{Host: host}); err != nil {
				t.Errorf("ListServices(%q): %v", host, err)
			}
		}
	})

	t.Run("system roots do not trust the test certificate", func(t *testing.T) {
		_, err := NewGRPCRequester().ListServices(ctx, request.GRPCConnectRequest{Host: "grpcs://" + tlsAddr})
		if err == nil || !strings.Contains(err.Error(), "certificate") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("plaintext call to a TLS server suggests grpcs", func(t *testing.T) {
		_, err := r.ListServices(ctx, request.GRPCConnectRequest{Host: tlsAddr})
		if err == nil || !strings.HasPrefix(err.Error(), "The server may require TLS — use grpcs://") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("describing a service names the TLS mismatch", func(t *testing.T) {
		_, err := r.DescribeService(ctx, request.GRPCConnectRequest{Host: tlsAddr}, "grpc.health.v1.Health")
		if err == nil || !strings.HasPrefix(err.Error(), "The server may require TLS — use grpcs://") {
			t.Fatalf("plaintext to TLS: err = %v", err)
		}
		_, err = r.DescribeService(ctx, request.GRPCConnectRequest{Host: "grpcs://" + plainAddr}, "grpc.health.v1.Health")
		if err == nil || !strings.HasPrefix(err.Error(), "The server may not use TLS — remove grpcs://") {
			t.Fatalf("TLS to plaintext: err = %v", err)
		}
	})

	t.Run("plaintext invoke with a proto file suggests grpcs", func(t *testing.T) {
		resp, err := r.Execute(ctx, request.GRPCExecuteRequest{
			Host:      tlsAddr,
			Service:   "grpc.health.v1.Health",
			Method:    "Check",
			ProtoPath: healthProto(t),
		})
		if resp != nil || err == nil || !strings.HasPrefix(err.Error(), "The server may require TLS — use grpcs://") {
			t.Fatalf("resp %+v err %v", resp, err)
		}
	})

	t.Run("grpcs to a plaintext server suggests removing it", func(t *testing.T) {
		_, err := r.ListServices(ctx, request.GRPCConnectRequest{Host: "grpcs://" + plainAddr})
		if err == nil || !strings.HasPrefix(err.Error(), "The server may not use TLS — remove grpcs://") {
			t.Fatalf("err = %v", err)
		}

		_, err = r.Execute(ctx, request.GRPCExecuteRequest{
			Host:    "grpcs://" + plainAddr,
			Service: "grpc.health.v1.Health",
			Method:  "Check",
		})
		if err == nil || !strings.HasPrefix(err.Error(), "The server may not use TLS — remove grpcs://") {
			t.Fatalf("Execute err = %v", err)
		}
	})
}

func healthProto(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "health.proto")
	src := `syntax = "proto3";
package grpc.health.v1;
message HealthCheckRequest { string service = 1; }
message HealthCheckResponse { int32 status = 1; }
service Health { rpc Check(HealthCheckRequest) returns (HealthCheckResponse); }
`
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tetiva test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:         true,

		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

func startHealthServer(t *testing.T, opts ...grpc.ServerOption) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer(opts...)
	healthpb.RegisterHealthServer(gs, health.NewServer())
	reflection.Register(gs)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return lis.Addr().String()
}
