package request_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func parseCurl(t *testing.T, cmd string) *request.ParsedCurl {
	t.Helper()
	got, err := request.ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl(%q): %v", cmd, err)
	}
	return got
}

func headerValueOf(items []entities.HeaderItem, key string) string {
	for _, h := range items {
		if strings.EqualFold(h.Key, key) {
			return h.Value
		}
	}
	return ""
}

type curlFormField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

func decodeFormFields(t *testing.T, body string) []curlFormField {
	t.Helper()
	var fields []curlFormField
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		t.Fatalf("body is not a form-field array: %v (%s)", err, body)
	}
	return fields
}

func warningWith(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// chromeCurl is a real "Copy as cURL" command from Chrome DevTools.
const chromeCurl = `curl 'https://api.example.com/v1/items?page=2' \
  -H 'accept: application/json, text/plain, */*' \
  -H 'accept-language: en-US,en;q=0.9' \
  -H 'authorization: Bearer eyJhbGci.eyJzdWIiOiIxIn0.sig' \
  -H 'content-type: application/json' \
  -H 'origin: https://app.example.com' \
  -H 'user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)' \
  --data-raw '{"filter":{"status":"active"},"limit":50}' \
  --compressed`

func TestParseCurl_ChromeCopyAsCurl(t *testing.T) {
	got := parseCurl(t, chromeCurl)

	if got.Method != "POST" {
		t.Errorf("method = %q, want POST", got.Method)
	}
	if got.URL != "https://api.example.com/v1/items?page=2" {
		t.Errorf("url = %q", got.URL)
	}
	if len(got.Headers) != 5 {
		t.Fatalf("headers = %d (%+v), want 5 (authorization moved to auth)", len(got.Headers), got.Headers)
	}
	if v := headerValueOf(got.Headers, "accept-language"); v != "en-US,en;q=0.9" {
		t.Errorf("accept-language = %q", v)
	}
	if v := headerValueOf(got.Headers, "user-agent"); v != "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" {
		t.Errorf("user-agent = %q", v)
	}
	if headerValueOf(got.Headers, "authorization") != "" {
		t.Error("authorization header must be consumed by auth")
	}
	if got.AuthType != entities.AuthTypeBearer {
		t.Errorf("auth type = %q, want bearer", got.AuthType)
	}
	if got.AuthData != `{"token":"eyJhbGci.eyJzdWIiOiIxIn0.sig","prefix":"Bearer"}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
	if got.BodyType != entities.BodyTypeJSON {
		t.Errorf("body type = %q, want json", got.BodyType)
	}
	if got.Body != `{"filter":{"status":"active"},"limit":50}` {
		t.Errorf("body = %q", got.Body)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

// The URL bar collapses newlines into spaces before the text reaches us,
// so the one-line form must parse identically.
func TestParseCurl_ChromeCopyAsCurlCollapsedToOneLine(t *testing.T) {
	multiline := parseCurl(t, chromeCurl)
	oneLine := parseCurl(t, strings.ReplaceAll(chromeCurl, "\n", " "))

	if !reflect.DeepEqual(multiline, oneLine) {
		t.Errorf("collapsed command parsed differently:\n multiline: %+v\n one line:  %+v", multiline, oneLine)
	}
}

func TestParseCurl_WindowsCaretStyle(t *testing.T) {
	cmd := "curl ^\"https://api.example.com/v1/items^\" ^\n" +
		"  -H ^\"accept: application/json^\" ^\n" +
		"  -H ^\"x-token: abc123^\" ^\n" +
		"  --compressed"

	got := parseCurl(t, cmd)

	if got.Method != "GET" {
		t.Errorf("method = %q, want GET", got.Method)
	}
	if got.URL != "https://api.example.com/v1/items" {
		t.Errorf("url = %q", got.URL)
	}
	if !hasHeader(got.Headers, "accept", "application/json") {
		t.Errorf("missing accept header: %+v", got.Headers)
	}
	if !hasHeader(got.Headers, "x-token", "abc123") {
		t.Errorf("missing x-token header: %+v", got.Headers)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_PostJSONWithContentType(t *testing.T) {
	got := parseCurl(t, `curl -X POST 'https://api.example.com/users' -H 'Content-Type: application/json' -d '{"name":"Alice"}'`)

	if got.Method != "POST" {
		t.Errorf("method = %q, want POST", got.Method)
	}
	if got.BodyType != entities.BodyTypeJSON {
		t.Errorf("body type = %q, want json", got.BodyType)
	}
	if got.Body != `{"name":"Alice"}` {
		t.Errorf("body = %q", got.Body)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_JSONBodyWithoutContentType(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/users -d '{"name":"Alice"}'`)

	if got.Method != "POST" {
		t.Errorf("method = %q, want POST", got.Method)
	}
	if got.BodyType != entities.BodyTypeJSON {
		t.Errorf("body type = %q, want json", got.BodyType)
	}
	if !warningWith(got.Warnings, "Content-Type") {
		t.Errorf("expected a warning about the missing Content-Type, got %v", got.Warnings)
	}
}

// A urlencoded body must land in the shape the send path re-encodes,
// so the imported request is actually sendable.
func TestParseCurl_UrlencodedBodyIsSendable(t *testing.T) {
	got := parseCurl(t, `curl -X POST https://api.example.com/login -H 'Content-Type: application/x-www-form-urlencoded' -d 'user=alice&pass=s3cret'`)

	if got.BodyType != entities.BodyTypeForm {
		t.Fatalf("body type = %q, want form", got.BodyType)
	}

	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP,
		Method: entities.HTTPMethod(got.Method), URL: got.URL, Headers: got.Headers,
		Body: got.Body, BodyType: got.BodyType,
		AuthType: got.AuthType, AuthData: got.AuthData,
	}
	requester := &mockRequester{response: &entities.Response{StatusCode: 200}}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, requester, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{},
		&noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil)

	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if requester.lastRequest.Body != "pass=s3cret&user=alice" {
		t.Errorf("sent body = %q, want the re-encoded form pairs", requester.lastRequest.Body)
	}
	if requester.lastRequest.Method != entities.MethodPOST {
		t.Errorf("sent method = %q", requester.lastRequest.Method)
	}
}

func TestParseCurl_FormWithFileAndTextFields(t *testing.T) {
	got := parseCurl(t, `curl -X POST https://api.example.com/upload -F 'name=alice' -F 'avatar=@/tmp/photos/a.jpg;type=image/jpeg'`)

	if got.BodyType != entities.BodyTypeForm {
		t.Fatalf("body type = %q, want form", got.BodyType)
	}

	fields := decodeFormFields(t, got.Body)
	if len(fields) != 2 {
		t.Fatalf("fields = %+v, want 2", fields)
	}
	if fields[0].Key != "name" || fields[0].Value != "alice" || fields[0].Type != "text" || !fields[0].Enabled {
		t.Errorf("text field = %+v", fields[0])
	}
	if fields[1].Key != "avatar" || fields[1].Value != "/tmp/photos/a.jpg" || fields[1].Type != "file" {
		t.Errorf("file field = %+v", fields[1])
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_UserFlag(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/me -u 'alice:s3cret'`)

	if got.AuthType != entities.AuthTypeBasic {
		t.Fatalf("auth type = %q, want basic", got.AuthType)
	}
	if got.AuthData != `{"username":"alice","password":"s3cret"}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
}

func TestParseCurl_UserFlagWithoutPassword(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/me -u alice`)

	if got.AuthData != `{"username":"alice","password":""}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
}

func TestParseCurl_UserFlagOverridesAuthorizationHeader(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/me -H 'Authorization: Bearer tok' -u alice:pw`)

	if got.AuthType != entities.AuthTypeBasic {
		t.Errorf("auth type = %q, want basic", got.AuthType)
	}
	if headerValueOf(got.Headers, "Authorization") != "" {
		t.Errorf("Authorization header must be dropped: %+v", got.Headers)
	}
	if !warningWith(got.Warnings, "-u credentials") {
		t.Errorf("expected an override warning, got %v", got.Warnings)
	}
}

func TestParseCurl_AuthorizationBasicHeader(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/me -H 'Authorization: Basic YWxpY2U6czNjcmV0'`)

	if got.AuthType != entities.AuthTypeBasic {
		t.Fatalf("auth type = %q, want basic", got.AuthType)
	}
	if got.AuthData != `{"username":"alice","password":"s3cret"}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
	if len(got.Headers) != 0 {
		t.Errorf("Authorization header must be consumed: %+v", got.Headers)
	}
}

func TestParseCurl_AuthorizationUnknownSchemeStaysHeader(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/me -H 'Authorization: Digest qop=auth'`)

	if got.AuthType != entities.AuthTypeNone {
		t.Errorf("auth type = %q, want none", got.AuthType)
	}
	if headerValueOf(got.Headers, "Authorization") != "Digest qop=auth" {
		t.Errorf("Authorization header must be kept: %+v", got.Headers)
	}
}

func TestParseCurl_AuthorizationBasicUndecodableStaysHeader(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/me -H 'Authorization: Basic not-base64!!'`)

	if got.AuthType != entities.AuthTypeNone {
		t.Errorf("auth type = %q, want none", got.AuthType)
	}
	if headerValueOf(got.Headers, "Authorization") != "Basic not-base64!!" {
		t.Errorf("Authorization header must be kept: %+v", got.Headers)
	}
	if !warningWith(got.Warnings, "could not be decoded") {
		t.Errorf("expected a decode warning, got %v", got.Warnings)
	}
}

func TestParseCurl_GetFlagMovesDataToQuery(t *testing.T) {
	got := parseCurl(t, `curl -G https://api.example.com/search?lang=en -d 'q=gopher' -d 'limit=10'`)

	if got.Method != "GET" {
		t.Errorf("method = %q, want GET", got.Method)
	}
	if got.URL != "https://api.example.com/search?lang=en&q=gopher&limit=10" {
		t.Errorf("url = %q", got.URL)
	}
	if got.BodyType != entities.BodyTypeNone || got.Body != "" {
		t.Errorf("body must be empty, got %q (%s)", got.Body, got.BodyType)
	}
}

func TestParseCurl_GetFlagKeepsFragmentLast(t *testing.T) {
	got := parseCurl(t, `curl -G 'https://api.example.com/search#results' -d 'q=gopher'`)

	if got.URL != "https://api.example.com/search?q=gopher#results" {
		t.Errorf("url = %q", got.URL)
	}
}

func TestParseCurl_GetFlagMergesWithExistingQueryBeforeFragment(t *testing.T) {
	got := parseCurl(t, `curl -G 'https://api.example.com/search?lang=en#results' -d 'q=gopher'`)

	if got.URL != "https://api.example.com/search?lang=en&q=gopher#results" {
		t.Errorf("url = %q", got.URL)
	}
}

// A '?' inside the fragment is not a query string; the appended data still needs its own.
func TestParseCurl_GetFlagIgnoresQuestionMarkInsideFragment(t *testing.T) {
	got := parseCurl(t, `curl -G 'https://api.example.com/search#a?b' -d 'q=gopher'`)

	if got.URL != "https://api.example.com/search?q=gopher#a?b" {
		t.Errorf("url = %q", got.URL)
	}
}

func TestParseCurl_JSONFlag(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/users --json '{"name":"Alice"}'`)

	if got.Method != "POST" {
		t.Errorf("method = %q, want POST", got.Method)
	}
	if got.BodyType != entities.BodyTypeJSON {
		t.Errorf("body type = %q, want json", got.BodyType)
	}
	if !hasHeader(got.Headers, "Content-Type", "application/json") {
		t.Errorf("missing Content-Type: %+v", got.Headers)
	}
	if !hasHeader(got.Headers, "Accept", "application/json") {
		t.Errorf("missing Accept: %+v", got.Headers)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_JSONFlagKeepsExplicitHeaders(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/users --json '{"a":1}' -H 'Accept: application/vnd.api+json'`)

	if v := headerValueOf(got.Headers, "Accept"); v != "application/vnd.api+json" {
		t.Errorf("Accept = %q, explicit header must win", v)
	}
}

func TestParseCurl_DataBinaryFile(t *testing.T) {
	got := parseCurl(t, `curl -X PUT https://api.example.com/blob --data-binary '@/tmp/payload.bin'`)

	if got.Method != "PUT" {
		t.Errorf("method = %q, want PUT", got.Method)
	}
	if got.BodyType != entities.BodyTypeBinary {
		t.Errorf("body type = %q, want binary", got.BodyType)
	}
	if got.Body != "/tmp/payload.bin" {
		t.Errorf("body = %q, want the file path", got.Body)
	}
}

func TestParseCurl_DataBinaryInlineIsNotAFile(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/x -H 'Content-Type: application/xml' --data-binary '<a>1</a>'`)

	if got.BodyType != entities.BodyTypeXML {
		t.Errorf("body type = %q, want xml", got.BodyType)
	}
	if got.Body != "<a>1</a>" {
		t.Errorf("body = %q", got.Body)
	}
}

// --data-urlencode percent-encodes the content part, so a value with '&' stays
// one parameter. Checked through -G because the query shows the exact wire form.
func TestParseCurl_DataUrlencodeForms(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "name and content", data: `q=a&b`, want: "q=a%26b"},
		{name: "content only", data: `hello world`, want: "hello+world"},
		{name: "leading equals drops the sign", data: `=a b`, want: "a+b"},
		{name: "space in content", data: `name=a b`, want: "name=a+b"},
		{name: "name is kept verbatim", data: `a@b=c d`, want: "a@b=c+d"},
		{name: "name is not re-encoded", data: `a%20b=c`, want: "a%20b=c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCurl(t, `curl -G https://x.test --data-urlencode '`+tt.data+`'`)

			if got.URL != "https://x.test?"+tt.want {
				t.Errorf("url = %q, want the query %q", got.URL, tt.want)
			}
		})
	}
}

func TestParseCurl_DataUrlencodeKeepsAmpersandInOneField(t *testing.T) {
	got := parseCurl(t, `curl https://x.test --data-urlencode 'q=a&b'`)

	fields := decodeFormFields(t, got.Body)
	if len(fields) != 1 {
		t.Fatalf("fields = %+v, want a single q field", fields)
	}
	if fields[0].Key != "q" || fields[0].Value != "a&b" {
		t.Errorf("field = %+v, want q=a&b", fields[0])
	}
}

func TestParseCurl_DataUrlencodeFileIsSkipped(t *testing.T) {
	got := parseCurl(t, `curl https://x.test --data-urlencode 'name@/tmp/f.txt' --data-urlencode '@/tmp/g.txt'`)

	if got.BodyType != entities.BodyTypeNone || got.Body != "" {
		t.Errorf("body = %q (%s), want nothing imported", got.Body, got.BodyType)
	}
	if !warningWith(got.Warnings, "--data-urlencode with a file") {
		t.Errorf("expected a file warning, got %v", got.Warnings)
	}
}

// A flag whose value we ignore must still consume that value, or the real URL
// is dropped and the value (often a secret) is imported as the URL instead.
func TestParseCurl_IgnoredValueFlagsKeepTheURL(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{name: "oauth2-bearer", cmd: `curl --oauth2-bearer SECRET https://api.example.com/x`},
		{name: "upload-file short", cmd: `curl -T /tmp/payload.bin https://api.example.com/x`},
		{name: "resolve", cmd: `curl --resolve api.example.com:443:127.0.0.1 https://api.example.com/x`},
		{name: "url-query", cmd: `curl --url-query 'a=1' https://api.example.com/x`},
		{name: "proxy-user short", cmd: `curl -U user:pw https://api.example.com/x`},
		{name: "unknown flag with a value", cmd: `curl --frobnicate SECRET https://api.example.com/x`},
		{name: "unknown short flag with a value", cmd: `curl -W SECRET https://api.example.com/x`},
		{name: "unknown flag before the url", cmd: `curl --frobnicate https://api.example.com/x`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCurl(t, tt.cmd)

			if got.URL != "https://api.example.com/x" {
				t.Errorf("url = %q", got.URL)
			}
			if strings.Contains(got.URL, "SECRET") || strings.Contains(got.AuthData, "SECRET") {
				t.Errorf("secret leaked into the request: url=%q auth=%q", got.URL, got.AuthData)
			}
		})
	}
}

func TestParseCurl_FormWithoutFilesWarnsAboutEncoding(t *testing.T) {
	got := parseCurl(t, `curl https://x.test -F 'a=1' -F 'b=2'`)

	if got.BodyType != entities.BodyTypeForm {
		t.Fatalf("body type = %q, want form", got.BodyType)
	}
	if !warningWith(got.Warnings, "multipart/form-data") {
		t.Errorf("expected a multipart warning, got %v", got.Warnings)
	}
}

// '<' makes curl read a text field from a file; importing it as an upload would
// send the wrong thing, so the field is dropped instead.
func TestParseCurl_FormFieldFromFileIsSkipped(t *testing.T) {
	got := parseCurl(t, `curl https://x.test -F 'note=</tmp/note.txt' -F 'a=1'`)

	fields := decodeFormFields(t, got.Body)
	if len(fields) != 1 || fields[0].Key != "a" {
		t.Errorf("fields = %+v, want only the a field", fields)
	}
	if !warningWith(got.Warnings, `"note"`) {
		t.Errorf("expected a warning for the skipped field, got %v", got.Warnings)
	}
}

func TestParseCurl_DataFileIsNotImported(t *testing.T) {
	got := parseCurl(t, `curl https://x.test -d @/tmp/body.json`)

	if got.Method != "POST" {
		t.Errorf("method = %q, want POST", got.Method)
	}
	if got.BodyType != entities.BodyTypeNone || got.Body != "" {
		t.Errorf("body = %q (%s), want nothing imported", got.Body, got.BodyType)
	}
	if !warningWith(got.Warnings, "-d reads the body from a file") {
		t.Errorf("expected a file warning, got %v", got.Warnings)
	}
}

func TestParseCurl_MultipleFileBodiesWarn(t *testing.T) {
	got := parseCurl(t, `curl https://x.test --data-binary @/tmp/a.bin --data-binary @/tmp/b.bin`)

	if got.Body != "/tmp/b.bin" {
		t.Errorf("body = %q, want the last file", got.Body)
	}
	if !warningWith(got.Warnings, "several file bodies") {
		t.Errorf("expected a warning about the dropped file, got %v", got.Warnings)
	}
}

func TestParseCurl_InlineDataMixedWithFileBodyWarns(t *testing.T) {
	got := parseCurl(t, `curl https://x.test --data-binary 'a=1' --data-binary @/tmp/a.bin`)

	if got.BodyType != entities.BodyTypeBinary || got.Body != "/tmp/a.bin" {
		t.Errorf("body = %q (%s), want the file", got.Body, got.BodyType)
	}
	if !warningWith(got.Warnings, "were mixed") {
		t.Errorf("expected a warning about the dropped data, got %v", got.Warnings)
	}
}

// Both paths are rejected by the send path, so the import has to say so.
func TestParseCurl_FilePathWarnings(t *testing.T) {
	relative := parseCurl(t, `curl https://x.test --data-binary @payload.bin`)
	if !warningWith(relative.Warnings, "is relative") {
		t.Errorf("expected a relative path warning, got %v", relative.Warnings)
	}

	traversal := parseCurl(t, `curl https://x.test --data-binary @/tmp/../etc/hosts`)
	if !warningWith(traversal.Warnings, `contains ".."`) {
		t.Errorf("expected a traversal warning, got %v", traversal.Warnings)
	}

	formPath := parseCurl(t, `curl https://x.test -F 'f=@/tmp/../etc/hosts'`)
	if !warningWith(formPath.Warnings, `contains ".."`) {
		t.Errorf("expected a traversal warning for -F, got %v", formPath.Warnings)
	}
}

func TestParseCurl_UnsupportedAndUnknownFlags(t *testing.T) {
	got := parseCurl(t, `curl -sSLk --compressed -o /tmp/out.txt -m 30 --frobnicate https://api.example.com/x -H 'X-Ok: 1'`)

	if got.Method != "GET" {
		t.Errorf("method = %q, want GET", got.Method)
	}
	if got.URL != "https://api.example.com/x" {
		t.Errorf("url = %q; -o argument must not be taken for the URL", got.URL)
	}
	if !hasHeader(got.Headers, "X-Ok", "1") {
		t.Errorf("headers = %+v", got.Headers)
	}
	if !warningWith(got.Warnings, "-k is not supported") {
		t.Errorf("expected a warning for -k, got %v", got.Warnings)
	}
	if !warningWith(got.Warnings, "-m is not supported") {
		t.Errorf("expected a warning for -m, got %v", got.Warnings)
	}
	if !warningWith(got.Warnings, "unknown flag --frobnicate") {
		t.Errorf("expected a warning for --frobnicate, got %v", got.Warnings)
	}
	if !warningWith(got.Warnings, "unknown flag -S") {
		t.Errorf("expected a warning for the bundled -S, got %v", got.Warnings)
	}
}

func TestParseCurl_UnsupportedMethodKeepsDefault(t *testing.T) {
	got := parseCurl(t, `curl -X TRACE https://api.example.com/x`)

	if got.Method != "GET" {
		t.Errorf("method = %q, want the default GET", got.Method)
	}
	if !warningWith(got.Warnings, `"TRACE"`) {
		t.Errorf("expected a warning about TRACE, got %v", got.Warnings)
	}
}

func TestParseCurl_EmptyHeaderFormIsSkipped(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/x -H 'X-Empty;' -H 'X-Ok: 1'`)

	if len(got.Headers) != 1 || !hasHeader(got.Headers, "X-Ok", "1") {
		t.Errorf("headers = %+v, want only X-Ok", got.Headers)
	}
	if !warningWith(got.Warnings, "X-Empty;") {
		t.Errorf("expected a warning for the empty-header form, got %v", got.Warnings)
	}
}

func TestParseCurl_HeaderValueKeepsColons(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/x -H 'X-Time: 12:30:00' -A 'my-agent/1.0' -e 'https://ref.example.com' -b 'sid=1; theme=dark'`)

	if v := headerValueOf(got.Headers, "X-Time"); v != "12:30:00" {
		t.Errorf("X-Time = %q", v)
	}
	if v := headerValueOf(got.Headers, "User-Agent"); v != "my-agent/1.0" {
		t.Errorf("User-Agent = %q", v)
	}
	if v := headerValueOf(got.Headers, "Referer"); v != "https://ref.example.com" {
		t.Errorf("Referer = %q", v)
	}
	if v := headerValueOf(got.Headers, "Cookie"); v != "sid=1; theme=dark" {
		t.Errorf("Cookie = %q", v)
	}
}

func TestParseCurl_CookieFileIsNotSupported(t *testing.T) {
	got := parseCurl(t, `curl https://api.example.com/x -b @cookies.txt`)

	if headerValueOf(got.Headers, "Cookie") != "" {
		t.Errorf("cookie file must not become a header: %+v", got.Headers)
	}
	if !warningWith(got.Warnings, "cookie file") {
		t.Errorf("expected a cookie file warning, got %v", got.Warnings)
	}
}

func TestParseCurl_AttachedAndInlineFlagValues(t *testing.T) {
	got := parseCurl(t, `curl -XPATCH --data='a=1' --url=https://api.example.com/x`)

	if got.Method != "PATCH" {
		t.Errorf("method = %q, want PATCH", got.Method)
	}
	if got.URL != "https://api.example.com/x" {
		t.Errorf("url = %q", got.URL)
	}
	if got.BodyType != entities.BodyTypeForm {
		t.Errorf("body type = %q, want form", got.BodyType)
	}
}

// curl defaults to http:// when the URL has no scheme, so the import must not
// silently upgrade the request to https.
func TestParseCurl_SchemelessURLGetsHTTP(t *testing.T) {
	got := parseCurl(t, `curl api.example.com/x`)

	if got.URL != "http://api.example.com/x" {
		t.Errorf("url = %q", got.URL)
	}
	if !warningWith(got.Warnings, "no scheme") {
		t.Errorf("expected a scheme warning, got %v", got.Warnings)
	}
}

func TestParseCurl_InvocationForms(t *testing.T) {
	tests := []struct {
		name    string
		command string
		ok      bool
	}{
		{"bare", `curl https://x.test`, true},
		{"windows exe", `curl.exe https://x.test`, true},
		{"absolute path", `/usr/bin/curl https://x.test`, true},
		{"path root", `/curl https://x.test`, true},
		{"relative path", `./curl https://x.test`, true},
		{"home path", `~/bin/curl https://x.test`, true},
		{"windows path", `C:\Windows\System32\curl.exe https://x.test`, true},
		{"windows forward slashes", `C:/tools/curl.exe https://x.test`, true},
		{"quoted windows path", `"C:\Program Files\curl\bin\curl.exe" https://x.test`, true},
		{"url ending in curl", `https://example.com/curl`, false},
		{"url ending in curl.exe", `https://example.com/curl.exe`, false},
		{"scheme-relative url", `//example.com/curl`, false},
		// A one-letter scheme is shaped exactly like a Windows drive letter.
		{"single-letter scheme url", `c://example.com/curl`, false},
		{"host-relative path", `example.com/curl`, false},
		{"word starting with curl", `curling https://x.test`, false},
		{"another program in the same directory", `./notcurl https://x.test`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := request.ParseCurl(tt.command)
			if tt.ok && errors.Is(err, request.ErrCurlNotCurl) {
				t.Fatalf("ParseCurl(%q) rejected the command: %v", tt.command, err)
			}
			if !tt.ok && !errors.Is(err, request.ErrCurlNotCurl) {
				t.Fatalf("ParseCurl(%q) = %v, want ErrCurlNotCurl", tt.command, err)
			}
		})
	}
}

func TestParseCurl_Errors(t *testing.T) {
	if _, err := request.ParseCurl("   "); !errors.Is(err, request.ErrCurlEmpty) {
		t.Errorf("empty input: got %v, want ErrCurlEmpty", err)
	}
	if _, err := request.ParseCurl("hello world"); !errors.Is(err, request.ErrCurlNotCurl) {
		t.Errorf("plain text: got %v, want ErrCurlNotCurl", err)
	}
	if _, err := request.ParseCurl("curl -X POST -H 'X-A: 1'"); !errors.Is(err, request.ErrCurlNoURL) {
		t.Errorf("no url: got %v, want ErrCurlNoURL", err)
	}
}

// Nothing is ever handed to a shell: substitutions stay literal text.
func TestParseCurl_NoShellSubstitution(t *testing.T) {
	got := parseCurl(t, `curl 'https://x.test' -d '$(rm -rf /)' -H "X-Cmd: ${HOME}"`)

	if got.Body != "$(rm -rf /)" {
		t.Errorf("body = %q, want the literal substitution text", got.Body)
	}
	if v := headerValueOf(got.Headers, "X-Cmd"); v != "${HOME}" {
		t.Errorf("X-Cmd = %q, want the literal variable text", v)
	}
}

func TestParseCurl_UnterminatedQuoteWarns(t *testing.T) {
	got := parseCurl(t, `curl https://x.test -H 'X-V: oops`)

	if !hasHeader(got.Headers, "X-V", "oops") {
		t.Errorf("headers = %+v", got.Headers)
	}
	if !warningWith(got.Warnings, "unterminated quote") {
		t.Errorf("expected an unterminated quote warning, got %v", got.Warnings)
	}
}

func TestParseCurl_Lexing(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{
			name: "single quotes are literal",
			cmd:  `curl https://x.test -H 'X-V: a\b$HOME'`,
			want: `a\b$HOME`,
		},
		{
			name: "quote escape idiom",
			cmd:  `curl https://x.test -H 'X-V: it'\''s'`,
			want: `it's`,
		},
		{
			name: "double quote escapes",
			cmd:  `curl https://x.test -H "X-V: say \"hi\" \$now"`,
			want: `say "hi" $now`,
		},
		{
			name: "backslash escape outside quotes",
			cmd:  `curl https://x.test -H X-V:a\'b`,
			want: `a'b`,
		},
		{
			name: "backslash newline continuation",
			cmd:  "curl https://x.test \\\n  -H 'X-V: ok'",
			want: "ok",
		},
		{
			name: "backslash space continuation before a flag",
			cmd:  `curl https://x.test \ -H 'X-V: ok'`,
			want: "ok",
		},
		{
			name: "backslash space continuation before the url",
			cmd:  `curl -H 'X-V: ok' \ https://x.test`,
			want: "ok",
		},
		{
			name: "escaped space inside an unquoted value",
			cmd:  `curl https://x.test -H X-V:John\ Doe`,
			want: "John Doe",
		},
		{
			name: "caret continuation",
			cmd:  "curl https://x.test ^\n  -H ^\"X-V: ok^\"",
			want: "ok",
		},
		{
			name: "caret escaped space inside a value",
			cmd:  "curl https://x.test -H X-V:John^ Doe",
			want: "John Doe",
		},
		{
			name: "ansi-c quoting for values with quotes",
			cmd:  `curl https://x.test -H $'X-V: it\'s\tfine'`,
			want: "it's\tfine",
		},
		{
			name: "ansi-c hex escape",
			cmd:  `curl https://x.test -H $'X-V: \x41\x42'`,
			want: "AB",
		},
		{
			name: "ansi-c octal escape",
			cmd:  `curl https://x.test -H $'X-V: \101\102'`,
			want: "AB",
		},
		{
			name: "ansi-c unicode escape",
			cmd:  `curl https://x.test -H $'X-V: caf\u00e9'`,
			want: "café",
		},
		{
			name: "ansi-c long unicode escape",
			cmd:  `curl https://x.test -H $'X-V: \U0001F600'`,
			want: "\U0001F600",
		},
		{
			name: "ansi-c unknown escape stays literal",
			cmd:  `curl https://x.test -H $'X-V: \q1'`,
			want: `\q1`,
		},
		{
			name: "plain double quotes",
			cmd:  `curl https://x.test -H "X-V: ok"`,
			want: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCurl(t, tt.cmd)
			if v := headerValueOf(got.Headers, "X-V"); v != tt.want {
				t.Errorf("X-V = %q, want %q", v, tt.want)
			}
		})
	}
}

func TestParseCurl_RoundTripBuildCurlJSON(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodPOST,
		URL:      "https://api.example.com/users",
		Headers:  []entities.HeaderItem{{Key: "X-Trace", Value: "abc", Enabled: true}},
		Body:     `{"name":"Alice"}`,
		BodyType: entities.BodyTypeJSON, AuthType: entities.AuthTypeNone,
	})
	res, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: testWorkspaceID})
	text := res.Command
	if err != nil {
		t.Fatalf("BuildCurl: %v", err)
	}

	got := parseCurl(t, text)

	if got.Method != "POST" {
		t.Errorf("method = %q, want POST", got.Method)
	}
	if got.URL != "https://api.example.com/users" {
		t.Errorf("url = %q", got.URL)
	}
	if !hasHeader(got.Headers, "X-Trace", "abc") {
		t.Errorf("headers = %+v", got.Headers)
	}
	if !hasHeader(got.Headers, "Content-Type", "application/json") {
		t.Errorf("headers = %+v", got.Headers)
	}
	if got.BodyType != entities.BodyTypeJSON || got.Body != `{"name":"Alice"}` {
		t.Errorf("body = %q (%s)", got.Body, got.BodyType)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_RoundTripBuildCurlBasicAuth(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/it's-fine", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeBasic, AuthData: `{"username":"bob","password":"s3cret"}`,
	})
	res, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: testWorkspaceID})
	text := res.Command
	if err != nil {
		t.Fatalf("BuildCurl: %v", err)
	}

	got := parseCurl(t, text)

	if got.URL != "https://api.example.com/it's-fine" {
		t.Errorf("url = %q", got.URL)
	}
	if got.AuthType != entities.AuthTypeBasic {
		t.Fatalf("auth type = %q, want basic", got.AuthType)
	}
	if got.AuthData != `{"username":"bob","password":"s3cret"}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
}

func TestParseCurl_DigestFlag(t *testing.T) {
	got := parseCurl(t, `curl --digest -u 'Mufasa:Circle Of Life' https://api.example.com/dir/index.html`)

	if got.AuthType != entities.AuthTypeDigest {
		t.Fatalf("auth type = %q, want digest", got.AuthType)
	}
	if got.AuthData != `{"username":"Mufasa","password":"Circle Of Life"}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_DigestWithoutCredentials(t *testing.T) {
	got := parseCurl(t, `curl --digest https://api.example.com/x`)

	if got.AuthType != entities.AuthTypeNone {
		t.Errorf("auth type = %q, want none", got.AuthType)
	}
	if !warningWith(got.Warnings, "--digest has no -u credentials") {
		t.Errorf("expected a missing-credentials warning, got %v", got.Warnings)
	}
}

func TestParseCurl_AWSSigV4(t *testing.T) {
	got := parseCurl(t, `curl --aws-sigv4 'aws:amz:eu-west-1:execute-api' `+
		`-u 'AKIDEXAMPLE:wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY' `+
		`-H 'x-amz-security-token: FQoDYXdzEJr' https://api.example.com/prod/orders`)

	if got.AuthType != entities.AuthTypeAWSSigV4 {
		t.Fatalf("auth type = %q, want aws_sigv4", got.AuthType)
	}
	want := `{"accessKeyId":"AKIDEXAMPLE","secretAccessKey":"wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",` +
		`"sessionToken":"FQoDYXdzEJr","region":"eu-west-1","service":"execute-api"}`
	if got.AuthData != want {
		t.Errorf("auth data = %q", got.AuthData)
	}
	if headerValueOf(got.Headers, "x-amz-security-token") != "" {
		t.Errorf("the session token header must move into the auth config: %+v", got.Headers)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", got.Warnings)
	}
}

func TestParseCurl_AWSSigV4SessionTokenBeforeAuthorization(t *testing.T) {
	got := parseCurl(t, `curl --aws-sigv4 'aws:amz:us-east-1:s3' -u 'AKID:SECRET' `+
		`-H 'x-amz-security-token: TOK' -H 'Authorization: Bearer leftover' https://api.example.com/x`)

	if got.AuthType != entities.AuthTypeAWSSigV4 {
		t.Fatalf("auth type = %q, want aws_sigv4", got.AuthType)
	}
	if headerValueOf(got.Headers, "Authorization") != "" {
		t.Errorf("Authorization header must be dropped: %+v", got.Headers)
	}
	if headerValueOf(got.Headers, "x-amz-security-token") != "" {
		t.Errorf("the session token header must move into the auth config: %+v", got.Headers)
	}
	if !warningWith(got.Warnings, "-u credentials replaced the Authorization header") {
		t.Errorf("expected the replaced-header warning, got %v", got.Warnings)
	}
}

func TestParseCurl_AWSSigV4WithoutRegion(t *testing.T) {
	got := parseCurl(t, `curl --aws-sigv4 aws:amz -u key:secret https://api.example.com/x`)

	if got.AuthType != entities.AuthTypeAWSSigV4 {
		t.Fatalf("auth type = %q, want aws_sigv4", got.AuthType)
	}
	if got.AuthData != `{"accessKeyId":"key","secretAccessKey":"secret","region":"","service":""}` {
		t.Errorf("auth data = %q", got.AuthData)
	}
	if !warningWith(got.Warnings, "has no region or service") {
		t.Errorf("expected a region warning, got %v", got.Warnings)
	}
}
