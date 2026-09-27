package wails

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

type stubExampleUsecase struct {
	createFn          func(context.Context, example.Create, example.CreateOpt) (*entities.ResponseExample, error)
	editFn            func(context.Context, example.Edit, example.EditOpt) (*entities.ResponseExample, error)
	deleteFn          func(context.Context, example.DeleteOpt) error
	listByRequestFn   func(context.Context, uuid.UUID) ([]*entities.ResponseExample, error)
	deleteByRequestFn func(context.Context, uuid.UUID, string) error
	moveFn            func(context.Context, uuid.UUID, uuid.UUID, string) error
}

func (s *stubExampleUsecase) Create(ctx context.Context, in example.Create, opt example.CreateOpt) (*entities.ResponseExample, error) {
	if s.createFn == nil {
		panic("createFn not set")
	}
	return s.createFn(ctx, in, opt)
}

func (s *stubExampleUsecase) Edit(ctx context.Context, in example.Edit, opt example.EditOpt) (*entities.ResponseExample, error) {
	if s.editFn == nil {
		panic("editFn not set")
	}
	return s.editFn(ctx, in, opt)
}

func (s *stubExampleUsecase) Delete(ctx context.Context, opt example.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubExampleUsecase) ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error) {
	if s.listByRequestFn == nil {
		panic("listByRequestFn not set")
	}
	return s.listByRequestFn(ctx, requestID)
}

func (s *stubExampleUsecase) DeleteByRequest(ctx context.Context, requestID uuid.UUID, userID string) error {
	if s.deleteByRequestFn == nil {
		panic("deleteByRequestFn not set")
	}
	return s.deleteByRequestFn(ctx, requestID, userID)
}

func (s *stubExampleUsecase) MoveToWorkspace(ctx context.Context, requestID, newWorkspaceID uuid.UUID, userID string) error {
	if s.moveFn == nil {
		panic("moveFn not set")
	}
	return s.moveFn(ctx, requestID, newWorkspaceID, userID)
}

func fixtureExample(requestID uuid.UUID, name string, version int) *entities.ResponseExample {
	now := time.Date(2026, 9, 25, 10, 11, 12, 0, time.UTC)
	return &entities.ResponseExample{
		ID:          uuid.New(),
		RequestID:   requestID,
		WorkspaceID: uuid.New(),
		Name:        name,
		StatusCode:  200,
		StatusText:  "OK",
		Headers:     []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:        `{"ok":true}`,
		ContentType: "application/json",
		Protocol:    entities.ProtocolHTTP,
		SortOrder:   3,
		Version:     version,
		CreatedBy:   defaultUserID,
		CreatedAt:   now,
		UpdatedBy:   defaultUserID,
		UpdatedAt:   now,
	}
}

func TestExampleService_List_Happy(t *testing.T) {
	reqID := uuid.New()
	e1 := fixtureExample(reqID, "200 OK", 1)
	e2 := fixtureExample(reqID, "404 Not Found", 2)
	stub := &stubExampleUsecase{
		listByRequestFn: func(_ context.Context, got uuid.UUID) ([]*entities.ResponseExample, error) {
			assert.Equal(t, reqID, got)
			return []*entities.ResponseExample{e1, e2}, nil
		},
	}

	res := NewExampleService(stub).List(reqID.String())

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, e1.ID.String(), res.Data[0].ID)
	assert.Equal(t, reqID.String(), res.Data[0].RequestID)
	assert.Equal(t, "404 Not Found", res.Data[1].Name)
	assert.Equal(t, 2, res.Data[1].Version)
}

func TestExampleService_List_EmptyIsArray(t *testing.T) {
	stub := &stubExampleUsecase{
		listByRequestFn: func(context.Context, uuid.UUID) ([]*entities.ResponseExample, error) {
			return nil, nil
		},
	}

	b, err := json.Marshal(NewExampleService(stub).List(uuid.NewString()))

	require.NoError(t, err)
	assert.Equal(t, `{"data":[]}`, string(b))
}

func TestExampleService_List_Error_BadUUID(t *testing.T) {
	res := NewExampleService(&stubExampleUsecase{}).List("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "requestId")
}

func TestExampleService_Create_Happy(t *testing.T) {
	reqID := uuid.New()
	created := fixtureExample(reqID, "201 Created", 1)
	stub := &stubExampleUsecase{
		createFn: func(_ context.Context, in example.Create, opt example.CreateOpt) (*entities.ResponseExample, error) {
			assert.Equal(t, example.Create{
				RequestID:   reqID,
				Name:        "201 Created",
				StatusCode:  201,
				StatusText:  "Created",
				Headers:     []entities.HeaderItem{{Key: "X-Id", Value: "7", Enabled: true}},
				Body:        "{}",
				ContentType: "application/json",
				Protocol:    entities.ProtocolGraphQL,
			}, in)
			assert.Equal(t, defaultUserID, opt.UserID)
			return created, nil
		},
	}

	res := NewExampleService(stub).Create(dto.CreateExampleRequest{
		RequestID:   reqID.String(),
		Name:        "201 Created",
		StatusCode:  201,
		StatusText:  "Created",
		Headers:     []dto.HeaderItemDTO{{Key: "X-Id", Value: "7", Enabled: true}},
		Body:        "{}",
		ContentType: "application/json",
		Protocol:    "graphql",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, dto.ExampleResponse{
		ID:          created.ID.String(),
		RequestID:   reqID.String(),
		Name:        "201 Created",
		StatusCode:  200,
		StatusText:  "OK",
		Headers:     []dto.HeaderItemDTO{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:        `{"ok":true}`,
		ContentType: "application/json",
		Protocol:    "http",
		SortOrder:   3,
		Version:     1,
		CreatedAt:   "2026-09-25T10:11:12Z",
		UpdatedAt:   "2026-09-25T10:11:12Z",
	}, res.Data)
}

func TestExampleService_Create_Error_BadUUID(t *testing.T) {
	res := NewExampleService(&stubExampleUsecase{}).Create(dto.CreateExampleRequest{RequestID: "nope", Name: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "requestId")
}

func TestExampleService_Create_PassesValidationFields(t *testing.T) {
	stub := &stubExampleUsecase{
		createFn: func(context.Context, example.Create, example.CreateOpt) (*entities.ResponseExample, error) {
			return nil, &domain.ValidationError{Fields: map[string]string{"request": "save the request before adding examples"}}
		},
	}

	res := NewExampleService(stub).Create(dto.CreateExampleRequest{RequestID: uuid.NewString(), Name: "x", Protocol: "http"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Equal(t, "save the request before adding examples", res.Error.Fields["request"])
}

func TestExampleService_Response_NilHeadersIsArray(t *testing.T) {
	e := fixtureExample(uuid.New(), "empty", 1)
	e.Headers = nil
	stub := &stubExampleUsecase{
		createFn: func(context.Context, example.Create, example.CreateOpt) (*entities.ResponseExample, error) {
			return e, nil
		},
	}

	res := NewExampleService(stub).Create(dto.CreateExampleRequest{RequestID: e.RequestID.String(), Name: "empty", Protocol: "http"})

	require.Nil(t, res.Error)
	require.NotNil(t, res.Data.Headers)
	assert.Empty(t, res.Data.Headers)
}

func TestExampleService_Edit_Happy(t *testing.T) {
	updated := fixtureExample(uuid.New(), "renamed", 5)
	stub := &stubExampleUsecase{
		editFn: func(_ context.Context, in example.Edit, opt example.EditOpt) (*entities.ResponseExample, error) {
			assert.Equal(t, example.Edit{
				Name:        "renamed",
				StatusCode:  500,
				StatusText:  "Internal Server Error",
				Headers:     []entities.HeaderItem{{Key: "Retry-After", Value: "5", Enabled: false}},
				Body:        "oops",
				ContentType: "text/plain",
			}, in)
			assert.Equal(t, example.EditOpt{ExampleID: updated.ID, UserID: defaultUserID, Version: 4}, opt)
			return updated, nil
		},
	}

	res := NewExampleService(stub).Edit(dto.EditExampleRequest{
		ID:          updated.ID.String(),
		Name:        "renamed",
		StatusCode:  500,
		StatusText:  "Internal Server Error",
		Headers:     []dto.HeaderItemDTO{{Key: "Retry-After", Value: "5", Enabled: false}},
		Body:        "oops",
		ContentType: "text/plain",
		Version:     4,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, updated.ID.String(), res.Data.ID)
	assert.Equal(t, 5, res.Data.Version)
}

func TestExampleService_Edit_Conflict(t *testing.T) {
	id := uuid.New()
	stub := &stubExampleUsecase{
		editFn: func(context.Context, example.Edit, example.EditOpt) (*entities.ResponseExample, error) {
			return nil, &domain.ConflictError{Entity: "example", ID: id.String()}
		},
	}

	res := NewExampleService(stub).Edit(dto.EditExampleRequest{ID: id.String(), Name: "x", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeConflict, res.Error.Code)
}

func TestExampleService_Edit_Error_BadUUID(t *testing.T) {
	res := NewExampleService(&stubExampleUsecase{}).Edit(dto.EditExampleRequest{ID: "nope"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestExampleService_Delete_Happy(t *testing.T) {
	id := uuid.New()
	stub := &stubExampleUsecase{
		deleteFn: func(_ context.Context, opt example.DeleteOpt) error {
			assert.Equal(t, example.DeleteOpt{ExampleID: id, UserID: defaultUserID, Version: 3}, opt)
			return nil
		},
	}

	res := NewExampleService(stub).Delete(dto.DeleteExampleRequest{ID: id.String(), Version: 3})

	require.Nil(t, res.Error)
}

func TestExampleService_Delete_NotFound(t *testing.T) {
	id := uuid.New()
	stub := &stubExampleUsecase{
		deleteFn: func(context.Context, example.DeleteOpt) error {
			return &domain.NotFoundError{Entity: "example", ID: id.String()}
		},
	}

	res := NewExampleService(stub).Delete(dto.DeleteExampleRequest{ID: id.String(), Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestExampleService_Delete_Error_BadUUID(t *testing.T) {
	res := NewExampleService(&stubExampleUsecase{}).Delete(dto.DeleteExampleRequest{ID: "nope"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestExampleService_ScanSecrets(t *testing.T) {
	svc := NewExampleService(&stubExampleUsecase{})

	res := svc.ScanSecrets(dto.ScanExampleRequest{
		Headers: []dto.HeaderItemDTO{{Key: "X-Debug", Value: "AKIAIOSFODNN7EXAMPLE", Enabled: true}},
		Body:    `{"refresh_token":"tGzv3JOkF0XG5Qx2TlKWIA"}`,
	})
	require.Nil(t, res.Error)
	assert.Equal(t, []string{"OAuth token", "AWS access key"}, res.Data)

	b, err := json.Marshal(svc.ScanSecrets(dto.ScanExampleRequest{Body: `{"id":1}`}))
	require.NoError(t, err)
	assert.Equal(t, `{"data":[]}`, string(b))
}
