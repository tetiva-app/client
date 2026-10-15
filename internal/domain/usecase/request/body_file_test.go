package request_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

var bodyFileTypes = []entities.BodyType{entities.BodyTypeBinary, entities.BodyTypeForm}

type bodyReadingRequester struct {
	body string
}

func (r *bodyReadingRequester) Execute(_ context.Context, req request.HTTPExecuteRequest) (*entities.Response, error) {
	data, err := io.ReadAll(req.BodyReader)
	if err != nil {
		return nil, err
	}
	r.body = string(data)
	return &entities.Response{StatusCode: 200, Headers: map[string][]string{}}, nil
}

func bodyFileRequest(t *testing.T, requester request.HTTPRequester, bodyType entities.BodyType, path string) (request.Usecase, uuid.UUID) {
	t.Helper()
	body := path
	if bodyType == entities.BodyTypeForm {
		data, err := json.Marshal([]map[string]any{{"key": "upload", "value": path, "type": "file", "enabled": true}})
		if err != nil {
			t.Fatal(err)
		}
		body = string(data)
	}
	repo := newMockRepo()
	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID:           id,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/upload",
		Body:         body,
		BodyType:     bodyType,
		AuthType:     entities.AuthTypeNone,
	}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, requester, nil, nil, &mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)
	return uc, id
}

func TestExecute_SendsBodyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("payload-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, bodyType := range bodyFileTypes {
		requester := &bodyReadingRequester{}
		uc, id := bodyFileRequest(t, requester, bodyType, path)

		if _, err := uc.Execute(t.Context(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
			t.Fatalf("%s body: %v", bodyType, err)
		}
		if !strings.Contains(requester.body, "payload-bytes") {
			t.Errorf("%s body sent %q", bodyType, requester.body)
		}
	}
}

func TestExecute_RefusesBodyFileThatIsNotLocalAndRegular(t *testing.T) {
	dir := t.TempDir()
	for path, want := range map[string]string{
		`/??/UNC/fileserver/share/payload.bin`: "openBodyFile: body files on network shares or URLs are not supported; copy the file to this computer",
		dir:                                    fmt.Sprintf("openBodyFile: %q is not a regular file", dir),
		filepath.Join(dir, "missing.bin"):      "openBodyFile: ",
	} {
		for _, bodyType := range bodyFileTypes {
			uc, id := bodyFileRequest(t, &mockRequester{}, bodyType, path)

			_, err := uc.Execute(t.Context(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s body %q: err = %v, want %q", bodyType, path, err, want)
			}
		}
	}
}
