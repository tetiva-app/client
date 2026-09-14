package sync

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/tetiva-app/client/internal/domain/entities"
	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

// ErrUpdateRequired marks an inbound entity written in a scheme this build does not
// know: retrying changes nothing until the app updates, so the syncer parks it.
var ErrUpdateRequired = errors.New("sync: entity needs a newer app version")

// checkAuthType keeps an unknown scheme out of the local database. An empty value
// comes from a peer that never set the field and stays an ordinary apply failure.
func checkAuthType(raw string) error {
	if raw == "" || entities.AuthType(raw).IsValid() {
		return nil
	}
	return fmt.Errorf("%w: unknown auth type %q", ErrUpdateRequired, raw)
}

func CollectionToProto(c *entities.Collection, operationID string) *syncv1.SyncEntity {
	grpcMetadata := make([]*syncv1.HeaderItem, len(c.GRPCMetadata))
	for i, h := range c.GRPCMetadata {
		grpcMetadata[i] = syncv1.HeaderItem_builder{
			Key: h.Key, Value: h.Value, Enabled: h.Enabled,
		}.Build()
	}

	var parentID string
	if c.ParentID != nil {
		parentID = c.ParentID.String()
	}

	return syncv1.SyncEntity_builder{
		EntityType:  syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		EntityId:    c.ID.String(),
		Version:     int32(c.Version),
		IsDeleted:   c.IsDelete,
		SortOrder:   int32(c.SortOrder),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
		CreatedAt:   timestamppb.New(c.CreatedAt),
		OperationId: operationID,
		Collection: syncv1.CollectionData_builder{
			ParentId:     parentID,
			Name:         c.Name,
			Description:  c.Description,
			AuthType:     string(c.AuthType),
			AuthData:     c.AuthData,
			GrpcMetadata: grpcMetadata,
			PreScript:    c.PreScript,
			PostScript:   c.PostScript,
		}.Build(),
	}.Build()
}

func RequestToProto(r *entities.Request, operationID string) *syncv1.SyncEntity {
	headers := make([]*syncv1.HeaderItem, len(r.Headers))
	for i, h := range r.Headers {
		headers[i] = syncv1.HeaderItem_builder{
			Key: h.Key, Value: h.Value, Enabled: h.Enabled,
		}.Build()
	}

	var grpcMetadata []*syncv1.HeaderItem
	for k, vals := range r.GRPCMetadata {
		for _, v := range vals {
			grpcMetadata = append(grpcMetadata, syncv1.HeaderItem_builder{
				Key: k, Value: v, Enabled: true,
			}.Build())
		}
	}

	return syncv1.SyncEntity_builder{
		EntityType:  syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:    r.ID.String(),
		Version:     int32(r.Version),
		IsDeleted:   r.IsDelete,
		SortOrder:   int32(r.SortOrder),
		UpdatedAt:   timestamppb.New(r.UpdatedAt),
		CreatedAt:   timestamppb.New(r.CreatedAt),
		OperationId: operationID,
		Request: syncv1.RequestData_builder{
			CollectionId:      r.CollectionID.String(),
			Name:              r.Name,
			Protocol:          string(r.Protocol),
			Method:            string(r.Method),
			Url:               r.URL,
			Headers:           headers,
			Body:              r.Body,
			BodyType:          string(r.BodyType),
			AuthType:          string(r.AuthType),
			AuthData:          r.AuthData,
			GrpcService:       r.GRPCService,
			GrpcMethod:        r.GRPCMethod,
			GrpcProtoPath:     r.GRPCProtoPath,
			GrpcMetadata:      grpcMetadata,
			GraphqlQuery:      r.GraphQLQuery,
			GraphqlVariables:  r.GraphQLVariables,
			GraphqlSchemaPath: r.GraphQLSchemaPath,
			GraphqlOperation:  r.GraphQLOperation,
			PreScript:         r.PreScript,
			PostScript:        r.PostScript,
			Description:       proto.String(r.Description), // presence: "" is a clear, absence is an older peer
		}.Build(),
	}.Build()
}

func EnvironmentToProto(e *entities.Environment, operationID string) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType:  syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT,
		EntityId:    e.ID.String(),
		Version:     int32(e.Version),
		IsDeleted:   e.IsDelete,
		UpdatedAt:   timestamppb.New(e.UpdatedAt),
		CreatedAt:   timestamppb.New(e.CreatedAt),
		OperationId: operationID,
		Environment: syncv1.EnvironmentData_builder{
			Name: e.Name,
		}.Build(),
	}.Build()
}

func VariableToProto(v *entities.Variable, operationID string) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType:  syncv1.EntityType_ENTITY_TYPE_VARIABLE,
		EntityId:    v.ID.String(),
		Version:     int32(v.Version),
		IsDeleted:   v.IsDelete,
		SortOrder:   int32(v.SortOrder),
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		CreatedAt:   timestamppb.New(v.CreatedAt),
		OperationId: operationID,
		Variable: syncv1.VariableData_builder{
			EnvironmentId: v.EnvironmentID.String(),
			Key:           v.Key,
			Value:         v.Value,
			IsSecret:      v.IsSecret,
			Enabled:       v.Enabled,
		}.Build(),
	}.Build()
}

func CollectionFromProto(e *syncv1.SyncEntity, workspaceID uuid.UUID) (*entities.Collection, error) {
	coll := e.GetCollection()
	if coll == nil {
		return nil, fmt.Errorf("missing collection data")
	}

	id, err := uuid.Parse(e.GetEntityId())
	if err != nil {
		return nil, fmt.Errorf("CollectionFromProto: invalid entity id %q: %w", e.GetEntityId(), err)
	}
	if err := checkAuthType(coll.GetAuthType()); err != nil {
		return nil, fmt.Errorf("CollectionFromProto: %w", err)
	}

	c := &entities.Collection{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        coll.GetName(),
		Description: coll.GetDescription(),
		AuthType:    entities.AuthType(coll.GetAuthType()),
		AuthData:    coll.GetAuthData(),
		PreScript:   coll.GetPreScript(),
		PostScript:  coll.GetPostScript(),
		SortOrder:   int(e.GetSortOrder()),
		Version:     int(e.GetVersion()),
		IsDelete:    e.GetIsDeleted(),
		CreatedBy:   "sync",
		UpdatedBy:   "sync",
	}

	if e.HasCreatedAt() {
		c.CreatedAt = e.GetCreatedAt().AsTime()
	}
	if e.HasUpdatedAt() {
		c.UpdatedAt = e.GetUpdatedAt().AsTime()
	}

	parentIDStr := coll.GetParentId()
	if parentIDStr != "" {
		pid, err := uuid.Parse(parentIDStr)
		if err != nil {
			return nil, fmt.Errorf("CollectionFromProto: invalid parent id %q: %w", parentIDStr, err)
		}
		c.ParentID = &pid
	}

	grpcMeta := coll.GetGrpcMetadata()
	c.GRPCMetadata = make([]entities.HeaderItem, len(grpcMeta))
	for i, h := range grpcMeta {
		c.GRPCMetadata[i] = entities.HeaderItem{Key: h.GetKey(), Value: h.GetValue(), Enabled: h.GetEnabled()}
	}

	return c, nil
}

func RequestFromProto(e *syncv1.SyncEntity) (*entities.Request, error) {
	rd := e.GetRequest()
	if rd == nil {
		return nil, fmt.Errorf("missing request data")
	}

	reqID, err := uuid.Parse(e.GetEntityId())
	if err != nil {
		return nil, fmt.Errorf("RequestFromProto: invalid entity id %q: %w", e.GetEntityId(), err)
	}
	collID, err := uuid.Parse(rd.GetCollectionId())
	if err != nil {
		return nil, fmt.Errorf("RequestFromProto: invalid collection id %q: %w", rd.GetCollectionId(), err)
	}
	if err := checkAuthType(rd.GetAuthType()); err != nil {
		return nil, fmt.Errorf("RequestFromProto: %w", err)
	}

	r := &entities.Request{
		ID:                reqID,
		CollectionID:      collID,
		Name:              rd.GetName(),
		Protocol:          entities.Protocol(rd.GetProtocol()),
		Method:            entities.HTTPMethod(rd.GetMethod()),
		URL:               rd.GetUrl(),
		Body:              rd.GetBody(),
		BodyType:          entities.BodyType(rd.GetBodyType()),
		AuthType:          entities.AuthType(rd.GetAuthType()),
		AuthData:          rd.GetAuthData(),
		GRPCService:       rd.GetGrpcService(),
		GRPCMethod:        rd.GetGrpcMethod(),
		GRPCProtoPath:     rd.GetGrpcProtoPath(),
		GraphQLQuery:      rd.GetGraphqlQuery(),
		GraphQLVariables:  rd.GetGraphqlVariables(),
		GraphQLSchemaPath: rd.GetGraphqlSchemaPath(),
		GraphQLOperation:  rd.GetGraphqlOperation(),
		PreScript:         rd.GetPreScript(),
		PostScript:        rd.GetPostScript(),
		Description:       rd.GetDescription(),
		SortOrder:         int(e.GetSortOrder()),
		Version:           int(e.GetVersion()),
		IsDelete:          e.GetIsDeleted(),
		CreatedBy:         "sync",
		UpdatedBy:         "sync",
	}

	if e.HasCreatedAt() {
		r.CreatedAt = e.GetCreatedAt().AsTime()
	}
	if e.HasUpdatedAt() {
		r.UpdatedAt = e.GetUpdatedAt().AsTime()
	}

	protoHeaders := rd.GetHeaders()
	r.Headers = make([]entities.HeaderItem, len(protoHeaders))
	for i, h := range protoHeaders {
		r.Headers[i] = entities.HeaderItem{Key: h.GetKey(), Value: h.GetValue(), Enabled: h.GetEnabled()}
	}

	r.GRPCMetadata = make(map[string][]string)
	for _, h := range rd.GetGrpcMetadata() {
		r.GRPCMetadata[h.GetKey()] = append(r.GRPCMetadata[h.GetKey()], h.GetValue())
	}

	return r, nil
}

func EnvironmentFromProto(e *syncv1.SyncEntity, workspaceID uuid.UUID) (*entities.Environment, error) {
	ed := e.GetEnvironment()
	if ed == nil {
		return nil, fmt.Errorf("missing environment data")
	}

	envID, err := uuid.Parse(e.GetEntityId())
	if err != nil {
		return nil, fmt.Errorf("EnvironmentFromProto: invalid entity id %q: %w", e.GetEntityId(), err)
	}

	env := &entities.Environment{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        ed.GetName(),
		Version:     int(e.GetVersion()),
		IsDelete:    e.GetIsDeleted(),
		CreatedBy:   "sync",
		UpdatedBy:   "sync",
	}

	if e.HasCreatedAt() {
		env.CreatedAt = e.GetCreatedAt().AsTime()
	}
	if e.HasUpdatedAt() {
		env.UpdatedAt = e.GetUpdatedAt().AsTime()
	}

	return env, nil
}

func VariableFromProto(e *syncv1.SyncEntity) (*entities.Variable, error) {
	vd := e.GetVariable()
	if vd == nil {
		return nil, fmt.Errorf("missing variable data")
	}

	varID, err := uuid.Parse(e.GetEntityId())
	if err != nil {
		return nil, fmt.Errorf("VariableFromProto: invalid entity id %q: %w", e.GetEntityId(), err)
	}
	envID, err := uuid.Parse(vd.GetEnvironmentId())
	if err != nil {
		return nil, fmt.Errorf("VariableFromProto: invalid environment id %q: %w", vd.GetEnvironmentId(), err)
	}

	v := &entities.Variable{
		ID:            varID,
		EnvironmentID: envID,
		Key:           vd.GetKey(),
		Value:         vd.GetValue(),
		IsSecret:      vd.GetIsSecret(),
		Enabled:       vd.GetEnabled(),
		SortOrder:     int(e.GetSortOrder()),
		Version:       int(e.GetVersion()),
		IsDelete:      e.GetIsDeleted(),
		CreatedBy:     "sync",
		UpdatedBy:     "sync",
	}

	if e.HasCreatedAt() {
		v.CreatedAt = e.GetCreatedAt().AsTime()
	}
	if e.HasUpdatedAt() {
		v.UpdatedAt = e.GetUpdatedAt().AsTime()
	}

	return v, nil
}
