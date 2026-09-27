package postman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// Importer is the portability.Importer for Postman Collection v2.1 files.
type Importer struct {
	collections CollectionCreator
	requests    RequestCreator
	examples    ExampleCreator
}

func NewImporter(collections CollectionCreator, requests RequestCreator, examples ExampleCreator) *Importer {
	return &Importer{collections: collections, requests: requests, examples: examples}
}

func (i *Importer) Detect(data []byte) bool {
	var probe struct {
		Info *struct {
			Schema string `json:"schema"`
		} `json:"info"`
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(data, &probe); err != nil || probe.Info == nil || probe.Info.Schema == "" {
		return false
	}
	return bytes.HasPrefix(bytes.TrimSpace(probe.Item), []byte("["))
}

// Preview runs the real import against recorders, so counts and warnings are the ones Import gives.
func (i *Importer) Preview(data []byte) (*portability.ImportPreview, error) {
	const funcName = "postman.Importer.Preview"

	rec := &recorder{}
	res, err := ImportCollection(context.Background(), data, ImportOpts{},
		collectionRecorder{rec}, requestRecorder{rec}, exampleRecorder{rec})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	var pc PostmanCollection
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return &portability.ImportPreview{
		Format:   portability.FormatPostman,
		Title:    pc.Info.Name,
		Folders:  res.FoldersCreated - 1,
		Requests: res.RequestsCreated,
		Examples: res.ExamplesCreated,
		Hosts:    portability.Hosts(rec.urls, nil),
		Scripts:  scriptPreviews(pc),
		Warnings: nonNil(res.Warnings),
	}, nil
}

func (i *Importer) Import(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	res, err := ImportCollection(ctx, data, ImportOpts{
		WorkspaceID: opt.WorkspaceID, UserID: opt.UserID, ParentID: opt.ParentID, IncludeScripts: opt.IncludeScripts,
	}, i.collections, i.requests, i.examples)
	if err != nil {
		return nil, err
	}
	return &portability.ImportResult{
		CollectionID: res.RootID,
		Folders:      res.FoldersCreated - 1,
		Requests:     res.RequestsCreated,
		Examples:     res.ExamplesCreated,
		Warnings:     nonNil(res.Warnings),
	}, nil
}

// recorder collects the URLs a dry run would have written: request URLs and OAuth 2.0 endpoints.
type recorder struct {
	urls []string
}

func (r *recorder) auth(authType entities.AuthType, data string) {
	if authType != entities.AuthTypeOAuth2 {
		return
	}
	fields, err := auth.ParseFields(data)
	if err != nil {
		return
	}
	for _, key := range []string{"tokenUrl", "authUrl", "deviceAuthUrl"} {
		if v := fields.Str(key); v != "" {
			r.urls = append(r.urls, v)
		}
	}
}

type collectionRecorder struct{ *recorder }

func (r collectionRecorder) Create(_ context.Context, in collection.Create, _ collection.CreateOpt) (*entities.Collection, error) {
	r.auth(in.AuthType, in.AuthData)
	return &entities.Collection{ID: uuid.New(), Name: in.Name, ParentID: in.ParentID, Version: 1}, nil
}

type requestRecorder struct{ *recorder }

func (r requestRecorder) Create(_ context.Context, in request.Create, _ request.CreateOpt) (*entities.Request, error) {
	r.urls = append(r.urls, in.URL)
	r.auth(in.AuthType, in.AuthData)
	return &entities.Request{ID: uuid.New(), Name: in.Name, Protocol: in.Protocol, Version: 1}, nil
}

type exampleRecorder struct{ *recorder }

func (exampleRecorder) Create(_ context.Context, in example.Create, _ example.CreateOpt) (*entities.ResponseExample, error) {
	return &entities.ResponseExample{ID: uuid.New(), RequestID: in.RequestID, Name: in.Name}, nil
}

// scriptPreviews lists the scripts an import with scripts on would keep, in the order collectScripts reads them.
func scriptPreviews(pc PostmanCollection) []portability.ScriptPreview {
	out := []portability.ScriptPreview{}
	var walk func(path string, events []PostmanEvent, items []PostmanItem)
	walk = func(path string, events []PostmanEvent, items []PostmanItem) {
		for _, ev := range events {
			code := strings.Join(ev.Script.Exec, "\n")
			if ev.Disabled || strings.TrimSpace(code) == "" {
				continue
			}
			switch ev.Listen {
			case "prerequest":
				out = append(out, portability.ScriptPreview{Path: path, Phase: "pre", Text: code})
			case "test":
				out = append(out, portability.ScriptPreview{Path: path, Phase: "post", Text: code})
			}
		}
		for _, it := range items {
			walk(path+" / "+it.Name, it.Event, derefItems(it.Item))
		}
	}
	walk(pc.Info.Name, pc.Event, pc.Item)
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
