package har

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain/entities"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return raw
}

func harOf(t *testing.T, entries ...map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"log": map[string]any{"version": "1.2", "entries": entries}})
	require.NoError(t, err)
	return raw
}

func get(url string) map[string]any {
	return map[string]any{"request": map[string]any{"method": "GET", "url": url, "headers": []any{}}}
}

func treeOf(t *testing.T, data []byte, fileName string) *portability.Tree {
	t.Helper()
	tr, err := parse(data, fileName)
	require.NoError(t, err)
	return tr
}

func TestDetect(t *testing.T) {
	imp := &Importer{}
	cases := map[string]struct {
		data string
		want bool
	}{
		"har":                 {`{"log":{"version":"1.2","creator":{"name":"x"},"entries":[]}}`, true},
		"har empty version":   {`{"log":{"version":"","entries":[{"request":{}}]}}`, true},
		"no version":          {`{"log":{"entries":[]}}`, false},
		"entries not a list":  {`{"log":{"version":"1.2","entries":{}}}`, false},
		"no log":              {`{"version":"1.2","entries":[]}`, false},
		"postman collection":  {`{"info":{"name":"P","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[]}`, false},
		"postman environment": {`{"name":"Dev","values":[],"_postman_variable_scope":"environment"}`, false},
		"tetiva snapshot":     {`{"format":"tetiva.collection-snapshot","version":1,"collection":{"name":"S","items":[]}}`, false},
		"hoppscotch":          {`{"v":12,"name":"H","folders":[],"requests":[]}`, false},
		"insomnia":            {`{"_type":"export","__export_format":4,"resources":[]}`, false},
		"openapi":             {`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{}}`, false},
		"array":               {`[{"log":{"version":"1.2","entries":[]}}]`, false},
		"not json":            {`log: {version: 1.2}`, false},
	}
	for name, tc := range cases {
		assert.Equal(t, tc.want, imp.Detect([]byte(tc.data)), name)
	}
	assert.True(t, imp.Detect(readFixture(t, "two-hosts.har")))
}

func TestBuild_FoldersPerHostInFirstSeenOrder(t *testing.T) {
	tr := treeOf(t, readFixture(t, "two-hosts.har"), "shop.example.com.har")

	assert.Equal(t, "shop.example.com", tr.Root.In.Name)
	assert.Empty(t, tr.Root.Requests)
	require.Len(t, tr.Root.Folders, 2)
	api, cdn := tr.Root.Folders[0], tr.Root.Folders[1]
	assert.Equal(t, "api.example.com", api.In.Name)
	assert.Equal(t, "cdn.example.com", cdn.In.Name)
	require.Len(t, api.Requests, 2)
	assert.Equal(t, "GET /v1/cart", api.Requests[0].In.Name)
	assert.Equal(t, "https://api.example.com/v1/cart?currency=EUR", api.Requests[0].In.URL)
	assert.Equal(t, "POST /v1/cart/items", api.Requests[1].In.Name)
	assert.Equal(t, entities.MethodPOST, api.Requests[1].In.Method)
	assert.Equal(t, "GET /img/logo.png", cdn.Requests[0].In.Name)
}

func TestBuild_OneHostNeedsNoFolder(t *testing.T) {
	tr := treeOf(t, harOf(t, get("https://api.example.com/a"), get("https://api.example.com"), get("http://api.example.com/c?x=1")), "")

	assert.Equal(t, "HAR import", tr.Root.In.Name)
	assert.Empty(t, tr.Root.Folders)
	var names []string
	for _, r := range tr.Root.Requests {
		names = append(names, r.In.Name)
		assert.Equal(t, entities.ProtocolHTTP, r.In.Protocol)
		assert.Equal(t, entities.AuthTypeNone, r.In.AuthType)
		assert.Equal(t, entities.BodyTypeNone, r.In.BodyType)
	}
	assert.Equal(t, []string{"GET /a", "GET /", "GET /c"}, names)
}

func TestBuild_HeadersLoseConnectionNoiseAndCookies(t *testing.T) {
	tr := treeOf(t, readFixture(t, "two-hosts.har"), "")

	cart := tr.Root.Folders[0].Requests[0].In
	assert.Equal(t, []entities.HeaderItem{
		{Key: "accept", Value: "application/json", Enabled: true},
		{Key: "authorization", Value: "Bearer abc.def.ghi", Enabled: true},
	}, cart.Headers)
	add := tr.Root.Folders[0].Requests[1].In
	assert.Equal(t, []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}}, add.Headers)
	assert.Contains(t, tr.Warnings, "cookie headers were not imported: add the cookies in the cookie manager if the requests need them")
}

func TestBuild_Bodies(t *testing.T) {
	post := func(mime, text string, params ...map[string]any) map[string]any {
		pd := map[string]any{"mimeType": mime, "text": text}
		if len(params) > 0 {
			pd["params"] = params
		}
		return map[string]any{"request": map[string]any{
			"method": "POST", "url": "https://api.example.com/x", "postData": pd,
			"headers": []any{map[string]any{"name": "Content-Type", "value": mime}},
		}}
	}
	cases := []struct {
		name      string
		entry     map[string]any
		bodyType  entities.BodyType
		body      string
		keepsType bool
	}{
		{"json", post("application/json", `{"a":1}`), entities.BodyTypeJSON, `{"a":1}`, true},
		{"vendor json", post("application/vnd.api+json", `{}`), entities.BodyTypeJSON, `{}`, true},
		{"xml", post("text/xml; charset=utf-8", `<a/>`), entities.BodyTypeXML, `<a/>`, true},
		{"text", post("text/plain", `hello`), entities.BodyTypeRaw, `hello`, true},
		{"urlencoded text", post("application/x-www-form-urlencoded", "q=a%20b&tag=x+y&empty"),
			entities.BodyTypeForm,
			`[{"enabled":true,"key":"q","type":"text","value":"a b"},{"enabled":true,"key":"tag","type":"text","value":"x y"},{"enabled":true,"key":"empty","type":"text","value":""}]`,
			false},
		{"urlencoded params only", post("application/x-www-form-urlencoded", "", map[string]any{"name": "q", "value": "1"}),
			entities.BodyTypeForm, `[{"enabled":true,"key":"q","type":"text","value":"1"}]`, false},
		{"multipart params", post("multipart/form-data; boundary=XYZ", "--XYZ...",
			map[string]any{"name": "title", "value": "Cat"},
			map[string]any{"name": "photo", "value": "binary", "fileName": "cat.png", "contentType": "image/png"}),
			entities.BodyTypeForm,
			`[{"enabled":true,"key":"title","type":"text","value":"Cat"},{"enabled":true,"key":"photo","type":"file","value":""}]`,
			false},
		{"multipart without params", post("multipart/form-data; boundary=XYZ", "--XYZ\r\n\r\nraw\r\n--XYZ--"),
			entities.BodyTypeRaw, "--XYZ\r\n\r\nraw\r\n--XYZ--", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := treeOf(t, harOf(t, tc.entry), "").Root.Requests[0].In

			assert.Equal(t, tc.bodyType, in.BodyType)
			assert.Equal(t, tc.body, in.Body)
			hasType := len(in.Headers) == 1 && in.Headers[0].Key == "Content-Type"
			assert.Equal(t, tc.keepsType, hasType)
		})
	}
}

func TestBuild_FilePartsAskToBePickedAgain(t *testing.T) {
	entry := map[string]any{"request": map[string]any{"method": "POST", "url": "https://api.example.com/x", "postData": map[string]any{
		"mimeType": "multipart/form-data; boundary=XYZ",
		"params": []any{
			map[string]any{"name": "a", "value": "x", "fileName": "a.png"},
			map[string]any{"name": "b", "value": "y", "fileName": "b.png"},
			map[string]any{"name": "c", "value": "z"},
		},
	}}}

	tr := treeOf(t, harOf(t, entry), "")

	assert.Equal(t, []string{"2 file fields were imported without the file; pick it again in the request body"}, tr.Warnings)
}

func TestBuild_MultipartWithoutFilesIsReported(t *testing.T) {
	entry := map[string]any{"request": map[string]any{"method": "POST", "url": "https://api.example.com/users", "postData": map[string]any{
		"mimeType": "multipart/form-data; boundary=XYZ",
		"params":   []any{map[string]any{"name": "name", "value": "Ann"}},
	}}}
	upload := map[string]any{"request": map[string]any{"method": "POST", "url": "https://api.example.com/avatars", "postData": map[string]any{
		"mimeType": "multipart/form-data; boundary=XYZ",
		"params":   []any{map[string]any{"name": "user", "value": "1"}, map[string]any{"name": "avatar", "fileName": "ann.png"}},
	}}}

	tr := treeOf(t, harOf(t, entry, upload), "")

	assert.Equal(t, []string{
		"1 file field was imported without the file; pick it again in the request body",
		"1 multipart request has no file field and will be sent as a URL-encoded form, not multipart: POST /users",
	}, tr.Warnings)
}

func TestBuild_ResponseBecomesAnExample(t *testing.T) {
	tr := treeOf(t, readFixture(t, "two-hosts.har"), "")

	cart := tr.Root.Folders[0].Requests[0]
	require.Len(t, cart.Examples, 1)
	ex := cart.Examples[0]
	assert.Equal(t, "200 OK", ex.Name)
	assert.Equal(t, 200, ex.StatusCode)
	assert.Equal(t, `{"items":[],"total":0}`, ex.Body)
	assert.Equal(t, "application/json; charset=utf-8", ex.ContentType)
	assert.Equal(t, entities.ProtocolHTTP, ex.Protocol)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "content-type", Value: "application/json; charset=utf-8", Enabled: true},
		{Key: "x-request-id", Value: "r-1", Enabled: true},
	}, ex.Headers)
	assert.Equal(t, "201 Created", tr.Root.Folders[0].Requests[1].Examples[0].Name)

	logo := tr.Root.Folders[1].Requests[0].Examples[0]
	assert.Empty(t, logo.Body)
	assert.Equal(t, "image/png", logo.ContentType)
	assert.Contains(t, tr.Warnings, "1 response body was left out of the examples: only JSON, XML and plain-text bodies up to 256 KB are kept")
}

func TestBuild_ExamplesKeepOnlyAPIBodiesAndFailedRequestsHaveNone(t *testing.T) {
	respond := func(url, mime string, size int) map[string]any {
		e := get(url)
		e["response"] = map[string]any{"status": 200, "content": map[string]any{"mimeType": mime, "text": strings.Repeat("a", size)}}
		return e
	}
	blocked := get("https://api.example.com/blocked")
	blocked["response"] = map[string]any{"status": 0, "statusText": "", "content": map[string]any{}}

	tr := treeOf(t, harOf(t,
		respond("https://api.example.com/big", "application/json", 256*1024+1),
		respond("https://api.example.com/edge", "application/problem+json", 256*1024),
		respond("https://api.example.com/feed", "application/atom+xml", 10),
		respond("https://api.example.com/plain", "text/plain; charset=utf-8", 10),
		respond("https://api.example.com/app.js", "application/javascript", 10),
		respond("https://api.example.com/page", "text/html", 10),
		blocked,
	), "")

	var bodies []int
	for _, r := range tr.Root.Requests[:6] {
		bodies = append(bodies, len(r.Examples[0].Body))
	}
	assert.Equal(t, []int{0, 256 * 1024, 10, 10, 0, 0}, bodies)
	assert.Empty(t, tr.Root.Requests[6].Examples)
	assert.Contains(t, tr.Warnings, "3 response bodies were left out of the examples: only JSON, XML and plain-text bodies up to 256 KB are kept")
}

func TestBuild_SkipsWhatIsNotHTTP(t *testing.T) {
	tr := treeOf(t, harOf(t,
		get("wss://api.example.com/live"),
		map[string]any{"_resourceType": "websocket", "request": map[string]any{"method": "GET", "url": "https://api.example.com/socket"}},
		get("data:image/png;base64,AAAA"),
		get("blob:https://app.example.com/1b2c"),
		get("chrome-extension://abcdef/content.js"),
		get("chrome-extension://abcdef/other.js"),
		get("https://api.example.com/kept"),
	), "")

	require.Len(t, tr.Root.Requests, 1)
	assert.Equal(t, []string{
		"2 WebSocket entries skipped: only HTTP requests are imported",
		"1 blob: entry skipped: only HTTP requests are imported",
		"2 chrome-extension: entries skipped: only HTTP requests are imported",
		"1 data: entry skipped: only HTTP requests are imported",
	}, tr.Warnings)
}

func TestBuild_SkipsPageAssets(t *testing.T) {
	typed := func(kind, url string) map[string]any {
		e := get(url)
		e["_resourceType"] = kind
		return e
	}
	tr := treeOf(t, harOf(t,
		typed("image", "https://cdn.example.com/logo.png"),
		typed("font", "https://cdn.example.com/inter.woff2"),
		typed("stylesheet", "https://cdn.example.com/app.css"),
		typed("script", "https://cdn.example.com/app.js"),
		typed("media", "https://cdn.example.com/intro.mp4"),
		typed("manifest", "https://app.example.com/manifest.json"),
		typed("texttrack", "https://cdn.example.com/intro.vtt"),
		typed("xhr", "https://api.example.com/a"),
		typed("fetch", "https://api.example.com/b"),
		typed("document", "https://api.example.com/"),
		typed("other", "https://api.example.com/c"),
		get("https://api.example.com/d"),
	), "")

	var names []string
	for _, r := range tr.Root.Requests {
		names = append(names, r.In.Name)
	}
	assert.Equal(t, []string{"GET /a", "GET /b", "GET /", "GET /c", "GET /d"}, names)
	assert.Equal(t, []string{"7 page asset entries skipped: images, scripts, stylesheets, fonts and media are not imported"}, tr.Warnings)
}

func TestBuild_ReportsCredentialsKeptFromTheCapture(t *testing.T) {
	withHeader := func(url, name, value string) map[string]any {
		e := get(url)
		e["request"].(map[string]any)["headers"] = []any{map[string]any{"name": name, "value": value}}
		return e
	}
	tr := treeOf(t, harOf(t,
		withHeader("https://api.example.com/a", "Authorization", "Bearer abc"),
		withHeader("https://api.example.com/b", "X-API-Key", "k1"),
		get("https://api.example.com/c?access_token=t1&page=2"),
		withHeader("https://api.example.com/d", "Access-Control-Request-Headers", "authorization"),
		withHeader("https://api.example.com/e", "Cookie", "session=s"),
		get("https://api.example.com/f?page=2"),
	), "")

	assert.Equal(t, "Bearer abc", tr.Root.Requests[0].In.Headers[0].Value)
	assert.Contains(t, tr.Warnings,
		"3 requests keep credentials from the capture in their headers or query parameters as plain values: move them to secret variables before sharing")
	one := treeOf(t, readFixture(t, "two-hosts.har"), "")
	assert.Contains(t, one.Warnings,
		"1 request keeps credentials from the capture in its headers or query parameters as plain values: move them to secret variables before sharing")
}

func TestPreview_CountsAndHosts(t *testing.T) {
	p, err := (&Importer{}).Preview(readFixture(t, "two-hosts.har"), "session.har")
	require.NoError(t, err)

	assert.Equal(t, portability.FormatHAR, p.Format)
	assert.Equal(t, "session", p.Title)
	assert.Equal(t, 2, p.Folders)
	assert.Equal(t, 3, p.Requests)
	assert.Equal(t, 3, p.Examples)
	assert.Equal(t, []string{"api.example.com", "cdn.example.com"}, p.Hosts)
	assert.Empty(t, p.EnvironmentNames)
	assert.NotNil(t, p.Scripts)
	assert.Len(t, p.Warnings, 5)
}

func TestPreview_RejectsABrokenFile(t *testing.T) {
	_, err := (&Importer{}).Preview([]byte(`{"log":{"version":"1.2","entries":[{"response":{"status":"200"}}]}}`), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}
