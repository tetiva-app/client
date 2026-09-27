package sync

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/tetiva-app/client/internal/domain/entities"
	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

func TestRequestFromProto_InvalidEntityID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   "not-a-uuid",
		Request: syncv1.RequestData_builder{
			CollectionId: uuid.NewString(),
			Name:         "x",
		}.Build(),
	}.Build()

	_, err := RequestFromProto(e)
	if err == nil {
		t.Fatal("expected error for malformed entity_id, got nil")
	}
}

func TestRequestFromProto_InvalidCollectionID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   uuid.NewString(),
		Request: syncv1.RequestData_builder{
			CollectionId: "garbage",
			Name:         "x",
		}.Build(),
	}.Build()

	_, err := RequestFromProto(e)
	if err == nil {
		t.Fatal("expected error for malformed collection_id, got nil")
	}
}

func TestCollectionFromProto_InvalidParentID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		EntityId:   uuid.NewString(),
		Collection: syncv1.CollectionData_builder{
			Name:     "x",
			ParentId: "nope",
		}.Build(),
	}.Build()

	_, err := CollectionFromProto(e, uuid.New())
	if err == nil {
		t.Fatal("expected error for malformed parent_id, got nil")
	}
}

func TestCollectionFromProto_InvalidEntityID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		EntityId:   "not-a-uuid",
		Collection: syncv1.CollectionData_builder{Name: "x"}.Build(),
	}.Build()

	_, err := CollectionFromProto(e, uuid.New())
	if err == nil {
		t.Fatal("expected error for malformed entity_id, got nil")
	}
}

func TestVariableFromProto_InvalidEnvironmentID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_VARIABLE,
		EntityId:   uuid.NewString(),
		Variable: syncv1.VariableData_builder{
			EnvironmentId: "bad",
			Key:           "k",
		}.Build(),
	}.Build()

	_, err := VariableFromProto(e)
	if err == nil {
		t.Fatal("expected error for malformed environment_id, got nil")
	}
}

func TestEnvironmentFromProto_InvalidEntityID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType:  syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT,
		EntityId:    "not-a-uuid",
		Environment: syncv1.EnvironmentData_builder{Name: "x"}.Build(),
	}.Build()

	_, err := EnvironmentFromProto(e, uuid.New())
	if err == nil {
		t.Fatal("expected error for malformed entity_id, got nil")
	}
}

func TestVariableFromProto_InvalidEntityID_ReturnsError(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_VARIABLE,
		EntityId:   "not-a-uuid",
		Variable:   syncv1.VariableData_builder{EnvironmentId: uuid.NewString(), Key: "k"}.Build(),
	}.Build()

	_, err := VariableFromProto(e)
	if err == nil {
		t.Fatal("expected error for malformed entity_id, got nil")
	}
}

func TestRequestFromProto_Valid(t *testing.T) {
	id, coll := uuid.NewString(), uuid.NewString()
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   id,
		Request:    syncv1.RequestData_builder{CollectionId: coll, Name: "ok"}.Build(),
	}.Build()

	r, err := RequestFromProto(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID.String() != id {
		t.Fatalf("id mismatch: got %s want %s", r.ID, id)
	}
	if r.CollectionID.String() != coll {
		t.Fatalf("collection id mismatch: got %s want %s", r.CollectionID, coll)
	}
}

func TestRequestToProto_SetsDescription(t *testing.T) {
	r := &entities.Request{ID: uuid.New(), CollectionID: uuid.New(), Name: "ok", Description: "# Docs"}
	e := RequestToProto(r, "op")
	if !e.GetRequest().HasDescription() || e.GetRequest().GetDescription() != "# Docs" {
		t.Fatalf("description not carried: has=%v got=%q", e.GetRequest().HasDescription(), e.GetRequest().GetDescription())
	}

	empty := RequestToProto(&entities.Request{ID: uuid.New(), CollectionID: uuid.New(), Name: "ok"}, "op")
	if !empty.GetRequest().HasDescription() {
		t.Fatal("an empty description must still assert presence: it is a deliberate clear")
	}
}

func TestRequestFromProto_Description(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   uuid.NewString(),
		Request: syncv1.RequestData_builder{
			CollectionId: uuid.NewString(), Name: "ok", Description: proto.String("x"),
		}.Build(),
	}.Build()

	r, err := RequestFromProto(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Description != "x" {
		t.Fatalf("description: got %q, want %q", r.Description, "x")
	}
}

func TestResponseExample_ProtoRoundTrip(t *testing.T) {
	created := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	in := &entities.ResponseExample{
		ID:          uuid.New(),
		RequestID:   uuid.New(),
		WorkspaceID: uuid.New(),
		Name:        "404 Not Found",
		StatusCode:  404,
		StatusText:  "Not Found",
		Headers: []entities.HeaderItem{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
			{Key: "X-Trace", Value: "abc", Enabled: false},
		},
		Body:        `{"error":"missing"}`,
		ContentType: "application/json",
		Protocol:    entities.ProtocolGraphQL,
		SortOrder:   7,
		Version:     3,
		CreatedAt:   created,
		UpdatedAt:   updated,
	}

	e := ResponseExampleToProto(in, "op-1")
	if e.GetEntityType() != syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE || e.GetOperationId() != "op-1" {
		t.Fatalf("envelope = %v/%q", e.GetEntityType(), e.GetOperationId())
	}
	if got := e.GetResponseExample().GetSortOrder(); got != 7 {
		t.Errorf("payload sort_order = %d, want 7: the server keeps only the payload's", got)
	}

	target := uuid.New()
	out, err := ResponseExampleFromProto(e, target)
	if err != nil {
		t.Fatalf("ResponseExampleFromProto: %v", err)
	}
	want := *in
	want.WorkspaceID = target
	want.CreatedBy, want.UpdatedBy = "sync", "sync"
	if !reflect.DeepEqual(*out, want) {
		t.Errorf("round trip:\n got %+v\nwant %+v", *out, want)
	}
}

func TestResponseExampleFromProto_Rejects(t *testing.T) {
	valid := func() *syncv1.ResponseExampleData {
		return syncv1.ResponseExampleData_builder{RequestId: uuid.NewString(), Name: "ok"}.Build()
	}
	cases := map[string]*syncv1.SyncEntity{
		"bad id": syncv1.SyncEntity_builder{
			EntityType: syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE, EntityId: "nope", ResponseExample: valid(),
		}.Build(),
		"bad request id": syncv1.SyncEntity_builder{
			EntityType: syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE, EntityId: uuid.NewString(),
			ResponseExample: syncv1.ResponseExampleData_builder{RequestId: "garbage", Name: "ok"}.Build(),
		}.Build(),
		"no payload": syncv1.SyncEntity_builder{
			EntityType: syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE, EntityId: uuid.NewString(),
		}.Build(),
	}
	for name, e := range cases {
		if _, err := ResponseExampleFromProto(e, uuid.New()); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestResponseExampleFromProto_Defaults(t *testing.T) {
	e := syncv1.SyncEntity_builder{
		EntityType:      syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE,
		EntityId:        uuid.NewString(),
		ResponseExample: syncv1.ResponseExampleData_builder{RequestId: uuid.NewString(), Name: "bare"}.Build(),
	}.Build()

	out, err := ResponseExampleFromProto(e, uuid.New())
	if err != nil {
		t.Fatalf("ResponseExampleFromProto: %v", err)
	}
	if out.Protocol != entities.ProtocolHTTP {
		t.Errorf("protocol = %q, want http for a peer that left it empty", out.Protocol)
	}
	if out.Headers == nil {
		t.Error("headers = nil, want an empty slice: the column rejects null JSON")
	}
}

func TestResponseExampleFromProto_MasksInboundHeaders(t *testing.T) {
	header := func(k, v string) *syncv1.HeaderItem {
		return syncv1.HeaderItem_builder{Key: k, Value: v, Enabled: true}.Build()
	}
	e := syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE,
		EntityId:   uuid.NewString(),
		ResponseExample: syncv1.ResponseExampleData_builder{
			RequestId: uuid.NewString(), Name: "peer",
			Headers: []*syncv1.HeaderItem{
				header("Authorization", "Bearer live"),
				header("X-Token", "Bearer {{token}}"),
				header("{{h}}", "literal"),
				header("Set-Cookie", "<redacted>"),
				header("Location", "https://h/cb?code=abc"),
				header("Accept", "text/plain"),
			},
		}.Build(),
	}.Build()

	out, err := ResponseExampleFromProto(e, uuid.New())
	if err != nil {
		t.Fatalf("ResponseExampleFromProto: %v", err)
	}
	want := []string{"Bearer <redacted>", "Bearer {{token}}", "<redacted>", "<redacted>", "https://h/cb?code=<redacted>", "text/plain"}
	for i, h := range out.Headers {
		if h.Value != want[i] {
			t.Errorf("%s = %q, want %q", h.Key, h.Value, want[i])
		}
	}
}

func TestResponseExampleToProto_MasksSensitiveHeaders(t *testing.T) {
	in := &entities.ResponseExample{
		ID: uuid.New(), RequestID: uuid.New(), Name: "leaky",
		Headers: []entities.HeaderItem{
			{Key: "Authorization", Value: "Bearer abc", Enabled: true},
			{Key: "Accept", Value: "text/plain", Enabled: true},
		},
	}

	headers := ResponseExampleToProto(in, "op").GetResponseExample().GetHeaders()
	if headers[0].GetValue() != "Bearer <redacted>" || headers[1].GetValue() != "text/plain" {
		t.Errorf("outgoing headers = %q, %q", headers[0].GetValue(), headers[1].GetValue())
	}
	if in.Headers[0].Value != "Bearer abc" {
		t.Error("the mapper rewrote the caller's entity")
	}
}
