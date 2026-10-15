package har

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const defaultName = "HAR import"

// A copied "accept-encoding: br" brings back bodies the client does not decode.
var droppedHeaders = map[string]bool{
	"host": true, "content-length": true, "connection": true, "keep-alive": true,
	"transfer-encoding": true, "accept-encoding": true, "content-encoding": true, "cookie": true,
}

var pageAssets = map[string]bool{
	"image": true, "font": true, "stylesheet": true, "script": true, "media": true, "manifest": true, "texttrack": true,
}

type Importer struct {
	w portability.Writer
}

func NewImporter(
	collections portability.CollectionCreator,
	requests portability.RequestCreator,
	examples portability.ExampleCreator,
	environments portability.EnvironmentCreator,
) *Importer {
	return &Importer{w: portability.Writer{
		Collections: collections, Requests: requests, Examples: examples, Environments: environments,
	}}
}

func (i *Importer) Detect(data []byte) bool {
	var probe struct {
		Log *struct {
			Version *string         `json:"version"`
			Entries json.RawMessage `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &probe); err != nil || probe.Log == nil || probe.Log.Version == nil {
		return false
	}
	return bytes.HasPrefix(bytes.TrimSpace(probe.Log.Entries), []byte("["))
}

func (i *Importer) Preview(data []byte, fileName string) (*portability.ImportPreview, error) {
	t, err := parse(data, fileName)
	if err != nil {
		return nil, fmt.Errorf("har.Importer.Preview: %w", err)
	}
	return t.Preview(portability.FormatHAR), nil
}

func (i *Importer) Import(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	t, err := parse(data, opt.FileName)
	if err != nil {
		return nil, fmt.Errorf("har.Importer.Import: %w", err)
	}
	return i.w.Write(ctx, t, opt)
}

func parse(data []byte, fileName string) (*portability.Tree, error) {
	var f harFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"content": "not a valid HAR file: " + err.Error()}}
	}
	return build(f.Log.Entries, portability.FileTitle(fileName, defaultName)), nil
}

func build(entries []entry, name string) *portability.Tree {
	t := &portability.Tree{Root: portability.Folder{In: collection.Create{Name: name}}}
	var (
		hosts   []portability.Folder
		skipped = map[string]int{}
		assets  int
		n       notes
	)
	for _, e := range entries {
		if pageAssets[e.ResourceType] {
			assets++
			continue
		}
		u, err := url.Parse(e.Request.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || e.ResourceType == "websocket" {
			skipped[skipKind(e, u)]++
			continue
		}
		r := mapRequest(e, u, &n)
		i := slices.IndexFunc(hosts, func(f portability.Folder) bool { return f.In.Name == u.Host })
		if i < 0 {
			hosts = append(hosts, portability.Folder{In: collection.Create{Name: u.Host}})
			i = len(hosts) - 1
		}
		hosts[i].Requests = append(hosts[i].Requests, r)
	}
	if len(hosts) == 1 {
		t.Root.Requests = hosts[0].Requests
	} else {
		t.Root.Folders = hosts
	}

	if assets > 0 {
		t.Warnings = append(t.Warnings, portability.Plural(assets, "page asset entry", "page asset entries")+
			" skipped: images, scripts, stylesheets, fonts and media are not imported")
	}
	for _, kind := range slices.Sorted(maps.Keys(skipped)) {
		t.Warnings = append(t.Warnings, fmt.Sprintf("%s skipped: only HTTP requests are imported", portability.Plural(skipped[kind], kind+" entry", kind+" entries")))
	}
	if n.cookies {
		t.Warnings = append(t.Warnings, "cookie headers were not imported: add the cookies in the cookie manager if the requests need them")
	}
	if n.credentials > 0 {
		t.Warnings = append(t.Warnings, portability.Plural(n.credentials,
			"request keeps credentials from the capture in its headers or query parameters",
			"requests keep credentials from the capture in their headers or query parameters")+
			" as plain values: move them to secret variables before sharing")
	}
	if n.files > 0 {
		t.Warnings = append(t.Warnings, fmt.Sprintf("%s imported without the file; pick it again in the request body",
			portability.Plural(n.files, "file field was", "file fields were")))
	}
	if len(n.textForms) > 0 {
		t.Warnings = append(t.Warnings, portability.URLEncodedForms(n.textForms))
	}
	if n.bodiesLeft > 0 {
		t.Warnings = append(t.Warnings, fmt.Sprintf("%s left out of the examples: only JSON, XML and plain-text bodies up to %d KB are kept",
			portability.Plural(n.bodiesLeft, "response body was", "response bodies were"), domain.MaxExampleBodyLen/1024))
	}
	return t
}

func skipKind(e entry, u *url.URL) string {
	if e.ResourceType == "websocket" || (u != nil && (u.Scheme == "ws" || u.Scheme == "wss")) {
		return "WebSocket"
	}
	if u != nil && u.Scheme != "" {
		return u.Scheme + ":"
	}
	return "unreadable URL"
}

type notes struct {
	cookies     bool
	credentials int
	files       int
	bodiesLeft  int
	textForms   []string
}

func mapRequest(e entry, u *url.URL, n *notes) portability.Request {
	method := entities.HTTPMethod(strings.ToUpper(e.Request.Method))
	if !method.IsValid() {
		method = entities.MethodGET
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	headers, cookie := mapHeaders(e.Request.Headers)
	n.cookies = n.cookies || cookie
	if carriesCredentials(headers, u) {
		n.credentials++
	}
	name := string(method) + " " + path
	bodyType, body := mapBody(e.Request.PostData, n)
	if bodyType == entities.BodyTypeForm {
		headers = slices.DeleteFunc(headers, func(h entities.HeaderItem) bool { return strings.EqualFold(h.Key, "Content-Type") })
		if pd := e.Request.PostData; strings.HasPrefix(strings.ToLower(pd.MimeType), "multipart/") &&
			!slices.ContainsFunc(pd.Params, func(p param) bool { return p.FileName != "" }) {
			n.textForms = append(n.textForms, name)
		}
	}
	in := request.Create{
		Name:     name,
		Protocol: entities.ProtocolHTTP,
		Method:   method,
		URL:      e.Request.URL,
		Headers:  headers,
		Body:     body,
		BodyType: bodyType,
		AuthType: entities.AuthTypeNone,
		AuthData: "{}",
	}
	r := portability.Request{In: in}
	if ex, ok := mapResponse(e.Response, n); ok {
		r.Examples = []example.Create{ex}
	}
	return r
}

func carriesCredentials(headers []entities.HeaderItem, u *url.URL) bool {
	if slices.ContainsFunc(headers, func(h entities.HeaderItem) bool { return secrets.IsSensitiveHeader(h.Key) }) {
		return true
	}
	for key := range u.Query() {
		if secrets.IsSensitiveQueryParam(key) {
			return true
		}
	}
	return false
}

func mapHeaders(in []nameValue) ([]entities.HeaderItem, bool) {
	out := []entities.HeaderItem{}
	cookie := false
	for _, h := range in {
		name := strings.ToLower(strings.TrimSpace(h.Name))
		if name == "cookie" {
			cookie = true
		}
		if strings.HasPrefix(name, ":") || droppedHeaders[name] {
			continue
		}
		out = append(out, entities.HeaderItem{Key: h.Name, Value: h.Value, Enabled: true})
	}
	return out, cookie
}

func mapBody(pd *postData, n *notes) (entities.BodyType, string) {
	if pd == nil || pd.Text == "" && len(pd.Params) == 0 {
		return entities.BodyTypeNone, ""
	}
	mime := strings.ToLower(pd.MimeType)
	switch {
	case strings.HasPrefix(mime, "application/x-www-form-urlencoded"):
		if pd.Text == "" {
			return entities.BodyTypeForm, formJSON(pd.Params)
		}
		return entities.BodyTypeForm, formJSON(parseURLEncoded(pd.Text))
	case strings.HasPrefix(mime, "multipart/form-data") && len(pd.Params) > 0:
		n.files += len(slices.DeleteFunc(slices.Clone(pd.Params), func(p param) bool { return p.FileName == "" }))
		return entities.BodyTypeForm, formJSON(pd.Params)
	case strings.Contains(mime, "json"):
		return entities.BodyTypeJSON, pd.Text
	case strings.Contains(mime, "xml"):
		return entities.BodyTypeXML, pd.Text
	default:
		return entities.BodyTypeRaw, pd.Text
	}
}

func parseURLEncoded(text string) []param {
	var out []param
	for pair := range strings.SplitSeq(text, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		out = append(out, param{Name: unescape(k), Value: unescape(v)})
	}
	return out
}

func unescape(s string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return s
}

func formJSON(params []param) string {
	fields := make([]map[string]any, 0, len(params))
	for _, p := range params {
		field := map[string]any{"key": p.Name, "value": p.Value, "type": "text", "enabled": true}
		if p.FileName != "" {
			field["value"], field["type"] = "", "file"
		}
		fields = append(fields, field)
	}
	data, _ := json.Marshal(fields)
	return string(data)
}

func mapResponse(r *harResponse, n *notes) (example.Create, bool) {
	if r == nil || r.Status <= 0 || r.Status > 999 {
		return example.Create{}, false
	}
	headers, _ := mapHeaders(r.Headers)
	contentType := r.Content.MimeType
	for _, h := range headers {
		if strings.EqualFold(h.Key, "Content-Type") {
			contentType = h.Value
		}
	}
	body := r.Content.Text
	if body != "" && (r.Content.Encoding == "base64" || len(body) > domain.MaxExampleBodyLen || !apiBody(contentType)) {
		body = ""
		n.bodiesLeft++
	}
	return example.Create{
		Name:        exampleName(r.Status, r.StatusText),
		StatusCode:  r.Status,
		StatusText:  r.StatusText,
		Headers:     headers,
		Body:        body,
		ContentType: contentType,
		Protocol:    entities.ProtocolHTTP,
	}, true
}

func apiBody(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "json") || strings.Contains(ct, "xml") || strings.HasPrefix(ct, "text/plain")
}

func exampleName(code int, text string) string {
	if text = strings.TrimSpace(text); text == "" {
		text = http.StatusText(code)
	}
	return strings.TrimSpace(strconv.Itoa(code) + " " + text)
}
