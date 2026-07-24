package request

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// internalCollectionReader is a minimal CollectionReader for whitebox tests.
type internalCollectionReader struct{}

func (internalCollectionReader) GetByID(_ context.Context, _ uuid.UUID) (*entities.Collection, error) {
	return nil, nil
}

func (internalCollectionReader) ListByWorkspace(_ context.Context, _ uuid.UUID) ([]*entities.Collection, error) {
	return nil, nil
}

// internalScriptResolver returns empty scripts.
type internalScriptResolver struct{}

func (internalScriptResolver) ResolvePreScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}
func (internalScriptResolver) ResolvePostScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

// newInternalUsecase builds a *usecase for whitebox tests of prepareHTTP.
func newInternalUsecase() *usecase {
	uc := NewUsecase(nil, nil, nil, nil, nil, nil, nil, &internalScriptResolver{}, nil, NewAuthResolver(internalCollectionReader{}), nil, nil).(*usecase)
	return uc
}

func TestPrepareHTTP_BasicGET(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/users",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}

	prep, sr, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if sr != nil {
		t.Errorf("expected nil scriptResult for no-script req, got %+v", sr)
	}
	if prep.URL != "https://api.example.com/users" {
		t.Errorf("URL: got %q", prep.URL)
	}
	if prep.Method != entities.MethodGET {
		t.Errorf("method: got %q", prep.Method)
	}
	if prep.Body != "" {
		t.Errorf("body: got %q, want empty", prep.Body)
	}
	if prep.BodyReader != nil {
		t.Errorf("BodyReader: got non-nil")
	}
}

func TestPrepareHTTP_VarSubstitution(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://{{host}}/items",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}

	prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{"host": "api.example.com"}, prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if prep.URL != "https://api.example.com/items" {
		t.Errorf("URL: got %q, want substituted", prep.URL)
	}
}

func TestPrepareHTTP_JSONBodyAddsContentType(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/users",
		Body:     `{"name":"Alice"}`,
		BodyType: entities.BodyTypeJSON,
		AuthType: entities.AuthTypeNone,
	}

	prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if got := prep.Headers["Content-Type"]; len(got) != 1 || got[0] != "application/json" {
		t.Errorf("Content-Type: got %v, want [application/json]", got)
	}
	if prep.Body != `{"name":"Alice"}` {
		t.Errorf("body: got %q", prep.Body)
	}
}

func TestPrepareHTTP_BinaryPathValidated(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/upload",
		Body:     "../etc/passwd",
		BodyType: entities.BodyTypeBinary,
		AuthType: entities.AuthTypeNone,
	}

	_, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, prepareOpt{})
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}
