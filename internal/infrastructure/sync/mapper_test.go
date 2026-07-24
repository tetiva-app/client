package sync

import (
	"testing"

	"github.com/google/uuid"
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
