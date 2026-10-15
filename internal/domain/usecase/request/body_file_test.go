package request_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
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
		`/??/UNC/fileserver/share/payload.bin`: "statBodyFile: body files on network shares or URLs are not supported; copy the file to this computer",
		dir:                                    fmt.Sprintf("statBodyFile: %q is not a regular file", dir),
		filepath.Join(dir, "missing.bin"):      "statBodyFile: ",
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

type countingRequester struct {
	sent, length int64
}

func (r *countingRequester) Execute(_ context.Context, req request.HTTPExecuteRequest) (*entities.Response, error) {
	n, err := io.Copy(io.Discard, req.BodyReader)
	if err != nil {
		return nil, err
	}
	r.sent, r.length = n, req.BodyLength
	return &entities.Response{StatusCode: 200, Headers: map[string][]string{}}, nil
}

func TestExecute_StreamsMultipartFileFromDisk(t *testing.T) {
	const size = 32 << 20
	path := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
	requester := &countingRequester{}
	uc, id := bodyFileRequest(t, requester, entities.BodyTypeForm, path)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := uc.Execute(t.Context(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)

	if requester.sent <= size || requester.sent != requester.length {
		t.Errorf("sent %d bytes with BodyLength %d for a %d-byte file", requester.sent, requester.length, size)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > size/4 {
		t.Errorf("Execute allocated %d MiB for a %d MiB file", grew>>20, size>>20)
	}
}

type replayingRequester struct {
	contentType   string
	first, second []byte
	length        int64
}

func (r *replayingRequester) Execute(_ context.Context, req request.HTTPExecuteRequest) (*entities.Response, error) {
	first, err := io.ReadAll(req.BodyReader)
	if err != nil {
		return nil, err
	}
	again, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	defer func() { _ = again.Close() }()
	second, err := io.ReadAll(again)
	if err != nil {
		return nil, err
	}
	r.contentType, r.first, r.second, r.length = req.Headers["Content-Type"][0], first, second, req.BodyLength
	return &entities.Response{StatusCode: 200, Headers: map[string][]string{}}, nil
}

func TestExecute_MultipartBodyCanBeSentAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("payload-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	requester := &replayingRequester{}
	uc, id := bodyFileRequest(t, requester, entities.BodyTypeForm, path)

	if _, err := uc.Execute(t.Context(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(requester.first, requester.second) {
		t.Errorf("GetBody produced another body:\n%s\n---\n%s", requester.first, requester.second)
	}
	if int64(len(requester.first)) != requester.length {
		t.Errorf("BodyLength %d, body is %d bytes", requester.length, len(requester.first))
	}
	_, params, err := mime.ParseMediaType(requester.contentType)
	if err != nil {
		t.Fatal(err)
	}
	form, err := multipart.NewReader(bytes.NewReader(requester.first), params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	files := form.File["upload"]
	if len(files) != 1 || files[0].Filename != "payload.bin" {
		t.Fatalf("upload part: %+v", files)
	}
	f, err := files[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if got, _ := io.ReadAll(f); string(got) != "payload-bytes" {
		t.Errorf("file part %q", got)
	}
}

func TestExecute_BinaryBodyCanBeSentAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("payload-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	requester := &replayingRequester{}
	uc, id := bodyFileRequest(t, requester, entities.BodyTypeBinary, path)

	if _, err := uc.Execute(t.Context(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatal(err)
	}

	if string(requester.first) != "payload-bytes" || string(requester.second) != "payload-bytes" {
		t.Errorf("sent %q, then %q", requester.first, requester.second)
	}
	if requester.length != int64(len("payload-bytes")) {
		t.Errorf("BodyLength %d, file is %d bytes", requester.length, len("payload-bytes"))
	}
}
