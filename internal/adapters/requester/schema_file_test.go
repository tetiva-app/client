package requester

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestReadSchemaFile(t *testing.T) {
	t.Run("reads a regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "api.graphql")
		if err := os.WriteFile(path, []byte("type Query { ok: Boolean }"), 0o600); err != nil {
			t.Fatal(err)
		}

		data, err := readSchemaFile(path)
		if err != nil || string(data) != "type Query { ok: Boolean }" {
			t.Fatalf("readSchemaFile = %q, %v", data, err)
		}
	})

	t.Run("refuses a directory", func(t *testing.T) {
		_, err := readSchemaFile(t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("refuses a file over the cap", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "huge.json")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(maxSchemaFileBytes + 1); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		_, err = readSchemaFile(path)
		if err == nil || !strings.Contains(err.Error(), "larger than 32 MiB") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("keeps not-exist detectable for proto import lookup", func(t *testing.T) {
		_, err := readSchemaFile(filepath.Join(t.TempDir(), "missing.proto"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("refuses a network path", func(t *testing.T) {
		_, err := readSchemaFile(`\\fileserver\share\api.graphql`)
		if !errors.Is(err, errNetworkSchemaPath) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestGRPCProtoPathRefusesNetworkPaths(t *testing.T) {
	r := NewGRPCRequester()
	connect := request.GRPCConnectRequest{ProtoPath: `//fileserver/share/api.proto`}

	if _, err := r.ListServices(t.Context(), connect); !errors.Is(err, errNetworkSchemaPath) {
		t.Errorf("ListServices err = %v", err)
	}
	if _, err := r.DescribeService(t.Context(), connect, "pkg.Svc"); !errors.Is(err, errNetworkSchemaPath) {
		t.Errorf("DescribeService err = %v", err)
	}
}

func TestGRPCProtoFileOverCapIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.proto")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxSchemaFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	for _, protoPath := range []string{path, dir} {
		_, err := NewGRPCRequester().ListServices(t.Context(), request.GRPCConnectRequest{ProtoPath: protoPath})
		if err == nil || !strings.Contains(err.Error(), "larger than 32 MiB") {
			t.Errorf("ListServices(%q) err = %v", protoPath, err)
		}
	}
}

func TestGraphQLSchemaPathRefusesNetworkPaths(t *testing.T) {
	_, err := NewGraphQLRequester(nil).Introspect(t.Context(), request.GraphQLIntrospectRequest{
		SchemaPath: `\\fileserver\share\api.graphql`,
	})
	if !errors.Is(err, errNetworkSchemaPath) || !strings.Contains(err.Error(), "; copy the file to this computer and pick the copy") {
		t.Fatalf("err = %v", err)
	}
}

func TestGRPCProtoImportsStillResolve(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"common/v1/page.proto": `syntax = "proto3";
package common.v1;
message Page { int32 size = 1; }
`,
		"users/v1/users.proto": `syntax = "proto3";
package users.v1;
import "common/v1/page.proto";
import "google/protobuf/timestamp.proto";
message ListRequest { common.v1.Page page = 1; google.protobuf.Timestamp since = 2; }
message ListResponse {}
service Users { rpc List(ListRequest) returns (ListResponse); }
`,
	}
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, protoPath := range []string{filepath.Join(root, "users", "v1", "users.proto"), root} {
		schema, err := NewGRPCRequester().ListServices(t.Context(), request.GRPCConnectRequest{ProtoPath: protoPath})
		if err != nil {
			t.Fatalf("ListServices(%q): %v", protoPath, err)
		}
		if len(schema.Services) != 1 || schema.Services[0].FullName != "users.v1.Users" {
			t.Errorf("ListServices(%q) services = %+v", protoPath, schema.Services)
		}
	}
}
