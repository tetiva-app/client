package wails

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type fakeCookieReader struct{ cookies []*http.Cookie }

func (f *fakeCookieReader) CookiesFor(_ context.Context, _ uuid.UUID, _ string) []*http.Cookie {
	return f.cookies
}

// stubCookieUsecase is a configurable cookie.Usecase double.
// Tests inject only the function fields they exercise; unset fields panic.
type stubCookieUsecase struct {
	addFn            func(context.Context, cookie.Add, cookie.Opt) (*entities.Cookie, error)
	editFn           func(context.Context, cookie.Edit, cookie.Opt) (*entities.Cookie, error)
	listFn           func(context.Context, cookie.ListOpt) ([]*entities.Cookie, error)
	deleteFn         func(context.Context, cookie.DeleteOpt) error
	deleteByDomainFn func(context.Context, uuid.UUID, string) (int, error)
	clearFn          func(context.Context, uuid.UUID) (int, error)
}

func (s *stubCookieUsecase) Add(ctx context.Context, in cookie.Add, opt cookie.Opt) (*entities.Cookie, error) {
	if s.addFn == nil {
		panic("addFn not set")
	}
	return s.addFn(ctx, in, opt)
}

func (s *stubCookieUsecase) Edit(ctx context.Context, in cookie.Edit, opt cookie.Opt) (*entities.Cookie, error) {
	if s.editFn == nil {
		panic("editFn not set")
	}
	return s.editFn(ctx, in, opt)
}

func (s *stubCookieUsecase) List(ctx context.Context, opt cookie.ListOpt) ([]*entities.Cookie, error) {
	if s.listFn == nil {
		panic("listFn not set")
	}
	return s.listFn(ctx, opt)
}

func (s *stubCookieUsecase) Delete(ctx context.Context, opt cookie.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubCookieUsecase) DeleteByDomain(ctx context.Context, ws uuid.UUID, domain string) (int, error) {
	if s.deleteByDomainFn == nil {
		panic("deleteByDomainFn not set")
	}
	return s.deleteByDomainFn(ctx, ws, domain)
}

func (s *stubCookieUsecase) Clear(ctx context.Context, ws uuid.UUID) (int, error) {
	if s.clearFn == nil {
		panic("clearFn not set")
	}
	return s.clearFn(ctx, ws)
}

func fixtureCookie(name, value, domainStr string) *entities.Cookie {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.Cookie{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		Domain:      domainStr,
		Path:        "/",
		Name:        name,
		Value:       value,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestCookieService_GetForURL_Happy(t *testing.T) {
	reader := &fakeCookieReader{cookies: []*http.Cookie{
		{Name: "session", Value: "abc", Domain: "api.example.com", Path: "/"},
	}}
	svc := NewCookieService(&stubCookieUsecase{}, request.CookieReader(reader))

	res := svc.GetForURL(uuid.NewString(), "https://api.example.com/x")

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 1)
	assert.Equal(t, "session", res.Data[0].Name)
}

func TestCookieService_GetForURL_Error_InvalidWorkspaceID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.GetForURL("not-a-uuid", "https://api.example.com")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestCookieService_List_Happy(t *testing.T) {
	wsID := uuid.New()
	c1 := fixtureCookie("a", "1", "x.test")
	c2 := fixtureCookie("b", "2", "y.test")
	uc := &stubCookieUsecase{
		listFn: func(_ context.Context, opt cookie.ListOpt) ([]*entities.Cookie, error) {
			assert.Equal(t, wsID, opt.WorkspaceID)
			return []*entities.Cookie{c1, c2}, nil
		},
	}
	svc := NewCookieService(uc, &fakeCookieReader{})

	res := svc.List(wsID.String())

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, "a", res.Data[0].Name)
	assert.Equal(t, "b", res.Data[1].Name)
}

func TestCookieService_List_Error_BadUUID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.List("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestCookieService_Add_Happy(t *testing.T) {
	wsID := uuid.New()
	expires := int64(1735689600) // 2025-01-01 UTC
	added := fixtureCookie("session", "xyz", "api.test")
	uc := &stubCookieUsecase{
		addFn: func(_ context.Context, in cookie.Add, _ cookie.Opt) (*entities.Cookie, error) {
			assert.Equal(t, wsID, in.WorkspaceID)
			assert.Equal(t, "api.test", in.Domain)
			assert.Equal(t, "session", in.Name)
			assert.Equal(t, "xyz", in.Value)
			require.NotNil(t, in.ExpiresAt)
			assert.Equal(t, expires, in.ExpiresAt.Unix())
			return added, nil
		},
	}
	svc := NewCookieService(uc, &fakeCookieReader{})

	res := svc.Add(dto.AddCookieRequest{
		WorkspaceID: wsID.String(),
		Domain:      "api.test",
		Name:        "session",
		Value:       "xyz",
		ExpiresAt:   &expires,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "session", res.Data.Name)
	assert.Equal(t, "xyz", res.Data.Value)
}

func TestCookieService_Add_Error_BadWorkspaceUUID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.Add(dto.AddCookieRequest{
		WorkspaceID: "not-a-uuid",
		Name:        "session",
		Value:       "xyz",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestCookieService_Edit_Happy(t *testing.T) {
	id := uuid.New()
	updated := fixtureCookie("session", "new", "api.test")
	updated.ID = id
	uc := &stubCookieUsecase{
		editFn: func(_ context.Context, in cookie.Edit, _ cookie.Opt) (*entities.Cookie, error) {
			assert.Equal(t, id, in.ID)
			assert.Equal(t, "session", in.Name)
			assert.Equal(t, "new", in.Value)
			return updated, nil
		},
	}
	svc := NewCookieService(uc, &fakeCookieReader{})

	res := svc.Edit(dto.EditCookieRequest{
		ID:     id.String(),
		Domain: "api.test",
		Name:   "session",
		Value:  "new",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, "new", res.Data.Value)
}

func TestCookieService_Edit_Error_BadUUID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.Edit(dto.EditCookieRequest{ID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestCookieService_Delete_Happy(t *testing.T) {
	id := uuid.New()
	uc := &stubCookieUsecase{
		deleteFn: func(_ context.Context, opt cookie.DeleteOpt) error {
			assert.Equal(t, id, opt.ID)
			return nil
		},
	}
	svc := NewCookieService(uc, &fakeCookieReader{})

	res := svc.Delete(id.String())

	require.Nil(t, res.Error)
}

func TestCookieService_Delete_Error_BadUUID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.Delete("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestCookieService_DeleteByDomain_Happy(t *testing.T) {
	wsID := uuid.New()
	uc := &stubCookieUsecase{
		deleteByDomainFn: func(_ context.Context, ws uuid.UUID, domainStr string) (int, error) {
			assert.Equal(t, wsID, ws)
			assert.Equal(t, "api.test", domainStr)
			return 3, nil
		},
	}
	svc := NewCookieService(uc, &fakeCookieReader{})

	res := svc.DeleteByDomain(wsID.String(), "api.test")

	require.Nil(t, res.Error)
	assert.Equal(t, 3, res.Data)
}

func TestCookieService_DeleteByDomain_Error_BadUUID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.DeleteByDomain("not-a-uuid", "api.test")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestCookieService_Clear_Happy(t *testing.T) {
	wsID := uuid.New()
	uc := &stubCookieUsecase{
		clearFn: func(_ context.Context, ws uuid.UUID) (int, error) {
			assert.Equal(t, wsID, ws)
			return 7, nil
		},
	}
	svc := NewCookieService(uc, &fakeCookieReader{})

	res := svc.Clear(wsID.String())

	require.Nil(t, res.Error)
	assert.Equal(t, 7, res.Data)
}

func TestCookieService_Clear_Error_BadUUID(t *testing.T) {
	svc := NewCookieService(&stubCookieUsecase{}, &fakeCookieReader{})

	res := svc.Clear("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}
