package request_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/har"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type snippetScriptResolver struct {
	pre   string
	calls int
}

func (r *snippetScriptResolver) ResolvePreScript(context.Context, *entities.Request) (string, error) {
	r.calls++
	return r.pre, nil
}

func (r *snippetScriptResolver) ResolvePostScript(context.Context, *entities.Request) (string, error) {
	r.calls++
	return "", nil
}

type countingCookieReader struct {
	cookies []*http.Cookie
	calls   int
}

func (c *countingCookieReader) CookiesFor(context.Context, uuid.UUID, string) []*http.Cookie {
	c.calls++
	return c.cookies
}

type uncachedProvider struct {
	peeks, acquisitions int
}

func (p *uncachedProvider) AccessToken(context.Context, entities.AuthOwner, auth.OAuth2Config) (string, error) {
	p.acquisitions++
	return "live-token", nil
}

func (p *uncachedProvider) Peek(context.Context, entities.AuthOwner, auth.OAuth2Config) (string, bool, error) {
	p.peeks++
	return "", false, nil
}

func (p *uncachedProvider) Status(context.Context, entities.AuthOwner, auth.OAuth2Config) (auth.TokenStatus, error) {
	return auth.TokenStatus{}, nil
}

func (p *uncachedProvider) Fetch(context.Context, entities.AuthOwner, auth.OAuth2Config) (*auth.Token, error) {
	p.acquisitions++
	return &auth.Token{AccessToken: "fetched-token"}, nil
}

func (p *uncachedProvider) Clear(context.Context, entities.AuthOwner) error { return nil }

type snippetDeps struct {
	env         *mockEnvResolver
	scripts     *snippetScriptResolver
	engine      *captureScriptEngine
	cookies     *countingCookieReader
	collections *mockCollectionReader
	uncached    *uncachedProvider
	provider    auth.Provider
}

func newSnippetDeps(vars map[string]string) *snippetDeps {
	uncached := &uncachedProvider{}
	return &snippetDeps{
		env:         &mockEnvResolver{vars: vars},
		scripts:     &snippetScriptResolver{},
		engine:      &captureScriptEngine{},
		cookies:     &countingCookieReader{},
		collections: fixtureCollections(),
		uncached:    uncached,
		provider:    uncached,
	}
}

func (d *snippetDeps) usecase() request.Usecase {
	return request.NewUsecase(newMockRepo(), &mockHistoryRepo{}, nil, nil, nil,
		d.env, d.engine, d.scripts, &noopVarPersister{},
		request.NewAuthResolver(d.collections), d.cookies, d.collections, nil, d.provider, nil, nil)
}

func (d *snippetDeps) build(t *testing.T, req *entities.Request, resolve bool) request.SnippetInput {
	t.Helper()
	in, err := d.usecase().BuildSnippetInput(context.Background(), req,
		request.BuildSnippetOpt{WorkspaceID: testWorkspaceID, ResolveVariables: resolve})
	require.NoError(t, err)
	return in
}

func (d *snippetDeps) buildWith(t *testing.T, req *entities.Request, opt request.BuildSnippetOpt) request.SnippetInput {
	t.Helper()
	opt.WorkspaceID = testWorkspaceID
	in, err := d.usecase().BuildSnippetInput(context.Background(), req, opt)
	require.NoError(t, err)
	return in
}

func (d *snippetDeps) buildHAR(t *testing.T, req *entities.Request, resolve bool) (*har.Request, []string) {
	t.Helper()
	in := d.build(t, req, resolve)
	require.NotNil(t, in.HAR)
	return in.HAR, in.Warnings
}

func snippetRequest(protocol entities.Protocol, method entities.HTTPMethod, rawURL string) *entities.Request {
	return &entities.Request{
		ID: uuid.New(), CollectionID: testCollectionID, Protocol: protocol,
		Method: method, URL: rawURL, BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}
}

func harHeader(r *har.Request, name string) (string, bool) {
	for _, h := range r.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value, true
		}
	}
	return "", false
}

const oauth2SnippetAuthData = `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token","clientId":"cid","clientSecret":"sec"}`

func TestBuildSnippetInput_Guards(t *testing.T) {
	t.Run("workspace_mismatch_short_circuits", func(t *testing.T) {
		for _, protocol := range []entities.Protocol{
			entities.ProtocolHTTP, entities.ProtocolGraphQL, entities.ProtocolGRPC, entities.ProtocolWebSocket,
		} {
			t.Run(string(protocol), func(t *testing.T) {
				d := newSnippetDeps(map[string]string{"host": "api.example.com"})
				d.scripts.pre = `pm.environment.set("a", "1")`
				d.cookies.cookies = []*http.Cookie{{Name: "session", Value: "abc"}}
				req := snippetRequest(protocol, entities.MethodGET, "https://{{host}}/x")

				_, err := d.usecase().BuildSnippetInput(context.Background(), req,
					request.BuildSnippetOpt{WorkspaceID: uuid.New(), ResolveVariables: true})

				var valErr *domain.ValidationError
				require.ErrorAs(t, err, &valErr)
				assert.Contains(t, valErr.Fields, "workspace")
				assert.Zero(t, d.env.calls)
				assert.Zero(t, d.scripts.calls)
				assert.Zero(t, d.cookies.calls)
			})
		}
	})

	t.Run("unknown_protocol_rejected", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest("smtp", entities.MethodGET, "https://api.example.com/x")

		_, err := d.usecase().BuildSnippetInput(context.Background(), req,
			request.BuildSnippetOpt{WorkspaceID: testWorkspaceID, ResolveVariables: true})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Fields, "protocol")
		assert.Zero(t, d.env.calls)
	})
}

func TestBuildSnippetInput_HTTP(t *testing.T) {
	t.Run("http_json_strips_jsonc", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/users")
		req.BodyType = entities.BodyTypeJSON
		req.Body = "{\n  // who\n  \"name\": \"neo\" /* id */\n}"

		in := d.build(t, req, true)

		assert.Equal(t, entities.ProtocolHTTP, in.Protocol)
		require.NotNil(t, in.HAR)
		require.NotNil(t, in.HAR.PostData)
		assert.Equal(t, "application/json", in.HAR.PostData.MimeType)
		assert.NotContains(t, in.HAR.PostData.Text, "//")
		assert.NotContains(t, in.HAR.PostData.Text, "/*")
		var body map[string]string
		require.NoError(t, json.Unmarshal([]byte(in.HAR.PostData.Text), &body))
		assert.Equal(t, map[string]string{"name": "neo"}, body)
		assert.Equal(t, "POST", in.HAR.Method)
		assert.Equal(t, "HTTP/1.1", in.HAR.HTTPVersion)
	})

	t.Run("http_unresolved_when_resolution_off", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"baseUrl": "https://api.example.com"})
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "{{baseUrl}}/u")

		r, _ := d.buildHAR(t, req, false)

		assert.Equal(t, "{{baseUrl}}/u", r.URL)
		assert.Zero(t, d.env.calls)
	})

	t.Run("http_unresolved_when_on_but_missing", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"other": "x"})
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "{{baseUrl}}/u")

		r, _ := d.buildHAR(t, req, true)

		assert.Equal(t, "{{baseUrl}}/u", r.URL)
	})

	t.Run("http_resolved_when_on", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"baseUrl": "https://api.example.com"})
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "{{baseUrl}}/u?page={{page}}")

		r, _ := d.buildHAR(t, req, true)

		assert.Equal(t, "https://api.example.com/u", r.URL)
		assert.Equal(t, []har.NameValue{{Name: "page", Value: "{{page}}"}}, r.QueryString)
	})

	t.Run("http_bearer_from_vars", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"token": "abc"})
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/me")
		req.AuthType = entities.AuthTypeBearer
		req.AuthData = `{"token":"{{token}}"}`

		on, _ := d.buildHAR(t, req, true)
		off, _ := d.buildHAR(t, req, false)

		got, _ := harHeader(on, "Authorization")
		assert.Equal(t, "Bearer abc", got)
		got, _ = harHeader(off, "Authorization")
		assert.Equal(t, "Bearer {{token}}", got)
	})

	t.Run("http_apikey_query_unresolved", func(t *testing.T) {
		cases := []struct {
			name      string
			authType  entities.AuthType
			authData  string
			resolve   bool
			wantQuery har.NameValue
		}{
			{"api_key off", entities.AuthTypeAPIKey, `{"key":"api_key","value":"{{key}}","addTo":"query"}`, false, har.NameValue{Name: "api_key", Value: "{{key}}"}},
			{"api_key on missing", entities.AuthTypeAPIKey, `{"key":"api_key","value":"{{key}}","addTo":"query"}`, true, har.NameValue{Name: "api_key", Value: "{{key}}"}},
			{"oauth2 query off", entities.AuthTypeOAuth2, `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token","clientId":"cid","addTo":"query"}`, false, har.NameValue{Name: "access_token", Value: "<token>"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				d := newSnippetDeps(map[string]string{"other": "x"})
				req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "{{baseUrl}}/u")
				req.AuthType, req.AuthData = tc.authType, tc.authData

				r, _ := d.buildHAR(t, req, tc.resolve)

				assert.Equal(t, "{{baseUrl}}/u", r.URL)
				assert.Equal(t, []har.NameValue{tc.wantQuery}, r.QueryString)
			})
		}
	})

	t.Run("http_host_port_query_auth", func(t *testing.T) {
		for _, resolve := range []bool{false, true} {
			d := newSnippetDeps(map[string]string{"other": "x"})
			req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}:{{port}}/a")
			req.AuthType = entities.AuthTypeAPIKey
			req.AuthData = `{"key":"api_key","value":"k1","addTo":"query"}`

			r, _ := d.buildHAR(t, req, resolve)

			assert.Equal(t, "https://{{host}}:{{port}}/a", r.URL, "resolve=%v", resolve)
			assert.Equal(t, []har.NameValue{{Name: "api_key", Value: "k1"}}, r.QueryString, "resolve=%v", resolve)
		}
	})

	t.Run("http_port_then_reference_query_auth", func(t *testing.T) {
		cases := []struct {
			vars    map[string]string
			resolve bool
			wantURL string
		}{
			{nil, false, "https://{{host}}:{{port}}{{basePath}}/u"},
			{map[string]string{"basePath": "/v1"}, true, "https://{{host}}:{{port}}/v1/u"},
			{map[string]string{"port": "8443"}, true, "https://{{host}}:8443{{basePath}}/u"},
		}
		for _, tc := range cases {
			d := newSnippetDeps(tc.vars)
			req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}:{{port}}{{basePath}}/u")
			req.AuthType = entities.AuthTypeAPIKey
			req.AuthData = `{"key":"api_key","value":"k1","addTo":"query"}`

			r, _ := d.buildHAR(t, req, tc.resolve)

			assert.Equal(t, tc.wantURL, r.URL)
			assert.Equal(t, []har.NameValue{{Name: "api_key", Value: "k1"}}, r.QueryString)
		}
	})

	t.Run("http_multipart_missing_file_ok", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/upload")
		req.BodyType = entities.BodyTypeForm
		req.Body = `[{"key":"name","value":"neo","type":"text","enabled":true},` +
			`{"key":"off","value":"x","type":"text","enabled":false},` +
			`{"key":"avatar","value":"/no/such/dir/avatar.png","type":"file","enabled":true}]`

		r, _ := d.buildHAR(t, req, true)

		require.NotNil(t, r.PostData)
		assert.Equal(t, "multipart/form-data", r.PostData.MimeType)
		assert.Equal(t, []har.Param{
			{Name: "name", Value: "neo"},
			{Name: "avatar", FileName: "avatar.png"},
		}, r.PostData.Params)
		_, hasCT := harHeader(r, "Content-Type")
		assert.False(t, hasCT)
	})

	t.Run("http_urlencoded_form", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"user": "neo"})
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/login")
		req.BodyType = entities.BodyTypeForm
		req.Body = `[{"key":"user","value":"{{user}}","type":"text","enabled":true},` +
			`{"key":"note","value":"","type":"text","enabled":true}]`

		r, _ := d.buildHAR(t, req, true)

		require.NotNil(t, r.PostData)
		assert.Equal(t, "application/x-www-form-urlencoded", r.PostData.MimeType)
		assert.Equal(t, []har.Param{{Name: "user", Value: "neo"}, {Name: "note"}}, r.PostData.Params)
	})

	t.Run("http_binary_placeholder_path", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/upload")
		req.BodyType = entities.BodyTypeBinary
		req.Body = "{{dir}}/a.bin"

		r, warnings := d.buildHAR(t, req, false)

		assert.Equal(t, "a.bin", r.BinaryFile)
		require.NotNil(t, r.PostData)
		assert.Equal(t, "application/octet-stream", r.PostData.MimeType)
		assert.Contains(t, warnings, "Binary body is shown as a file reference")
	})

	t.Run("http_prescript_warning", func(t *testing.T) {
		d := newSnippetDeps(nil)
		d.scripts.pre = `pm.request.headers.add({key: "X-Sig", value: "1"})`
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")

		_, warnings := d.buildHAR(t, req, true)

		assert.Contains(t, warnings, "Pre-request script is not applied to snippets")
		assert.Zero(t, d.engine.preCalls)
	})

	t.Run("http_digest_note", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
		req.AuthType = entities.AuthTypeDigest
		req.AuthData = `{"username":"u","password":"p"}`

		r, warnings := d.buildHAR(t, req, true)

		assert.Equal(t, "digest", r.AuthNote)
		_, hasAuth := harHeader(r, "Authorization")
		assert.False(t, hasAuth)
		assert.False(t, warningWith(warnings, "Digest"), "warnings: %v", warnings)
	})

	t.Run("http_cookies_added_when_resolving", func(t *testing.T) {
		d := newSnippetDeps(nil)
		d.cookies.cookies = []*http.Cookie{{Name: "session", Value: "abc"}, {Name: "csrf", Value: "tok"}}
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
		req.Headers = []entities.HeaderItem{{Key: "cookie", Value: "pref=1", Enabled: true}}

		on, _ := d.buildHAR(t, req, true)
		got, _ := harHeader(on, "Cookie")
		assert.Equal(t, "pref=1; session=abc; csrf=tok", got)

		calls := d.cookies.calls
		off, _ := d.buildHAR(t, req, false)
		got, _ = harHeader(off, "Cookie")
		assert.Equal(t, "pref=1", got)
		assert.Equal(t, calls, d.cookies.calls)
	})

	t.Run("http_oauth2_placeholder_when_off", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
		req.AuthType, req.AuthData = entities.AuthTypeOAuth2, oauth2SnippetAuthData

		r, warnings := d.buildHAR(t, req, false)

		got, _ := harHeader(r, "Authorization")
		assert.Equal(t, "Bearer <token>", got)
		assert.Contains(t, warnings, "OAuth 2.0 token is not included")
		assert.Zero(t, d.uncached.peeks+d.uncached.acquisitions)
	})

	t.Run("http_oauth2_uncached_warns_when_on", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
		req.AuthType, req.AuthData = entities.AuthTypeOAuth2, oauth2SnippetAuthData

		r, warnings := d.buildHAR(t, req, true)

		_, hasAuth := harHeader(r, "Authorization")
		assert.False(t, hasAuth)
		assert.True(t, warningWith(warnings, "No cached OAuth 2.0 token"), "warnings: %v", warnings)
		assert.Equal(t, 1, d.uncached.peeks)
		assert.Zero(t, d.uncached.acquisitions)
	})

	t.Run("http_oauth2_cached_token_with_unresolved_field", func(t *testing.T) {
		const authData = `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token","clientId":"cid","scope":"{{scope}}"}`
		d := newSnippetDeps(map[string]string{"other": "x"})
		store := newMemTokenStore()
		d.provider = auth.NewProvider(store, nil, nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
		req.AuthType, req.AuthData = entities.AuthTypeOAuth2, authData
		store.seed(requestOwner(req.ID), configHashOf(t, authData), "cached-at")

		r, _ := d.buildHAR(t, req, true)

		got, _ := harHeader(r, "Authorization")
		assert.Equal(t, "Bearer cached-at", got)
	})

	t.Run("unsafe_name_roundtrip", func(t *testing.T) {
		const unsafe = "{{x'; printf injected; #}}"
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/"+unsafe+"/u")
		req.Headers = []entities.HeaderItem{{Key: "X-Odd", Value: "{{a b}} {{$HOME}}", Enabled: true}}
		req.AuthType = entities.AuthTypeAPIKey
		req.AuthData = `{"key":"api_key","value":"k1","addTo":"query"}`

		r, _ := d.buildHAR(t, req, false)

		assert.Equal(t, "https://api.example.com/"+unsafe+"/u", r.URL)
		got, _ := harHeader(r, "X-Odd")
		assert.Equal(t, "{{a b}} {{$HOME}}", got)
	})
}

func TestBuildSnippetInput_GraphQL(t *testing.T) {
	graphqlRequest := func() *entities.Request {
		req := snippetRequest(entities.ProtocolGraphQL, entities.MethodPOST, "https://api.example.com/graphql")
		req.GraphQLQuery = "query GetUser($id: ID!) { user(id: $id) { name } }"
		req.GraphQLVariables = `{"id": "{{userId}}"}`
		req.GraphQLOperation = "GetUser"
		return req
	}

	t.Run("graphql_body_and_content_type", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"userId": "42"})

		in := d.build(t, graphqlRequest(), true)

		assert.Equal(t, entities.ProtocolGraphQL, in.Protocol)
		require.NotNil(t, in.HAR)
		assert.Equal(t, "POST", in.HAR.Method)
		assert.Equal(t, "https://api.example.com/graphql", in.HAR.URL)
		ct, _ := harHeader(in.HAR, "Content-Type")
		assert.Equal(t, "application/json", ct)
		require.NotNil(t, in.HAR.PostData)
		assert.Equal(t, "application/json", in.HAR.PostData.MimeType)
		assert.Equal(t,
			`{"query":"query GetUser($id: ID!) { user(id: $id) { name } }","variables":{"id":"42"},"operationName":"GetUser"}`,
			in.HAR.PostData.Text)
		assert.Empty(t, in.Warnings)
	})

	t.Run("graphql_html_characters_literal", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := graphqlRequest()
		req.GraphQLQuery = `{ find(q: "a & b <c>") { id } }`
		req.GraphQLVariables = ""
		req.GraphQLOperation = ""

		r, _ := d.buildHAR(t, req, false)

		require.NotNil(t, r.PostData)
		assert.Equal(t, `{"query":"{ find(q: \"a & b <c>\") { id } }"}`, r.PostData.Text)
	})

	t.Run("graphql_invalid_variables_lenient", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := graphqlRequest()
		req.GraphQLVariables = `{"id": {{userId}}}`

		r, warnings := d.buildHAR(t, req, false)

		require.NotNil(t, r.PostData)
		assert.Contains(t, r.PostData.Text, `"variables":{"id": {{userId}}}`)
		assert.Contains(t, warnings, "GraphQL variables are not valid JSON; shown as written")
	})

	t.Run("graphql_cookies", func(t *testing.T) {
		d := newSnippetDeps(nil)
		d.cookies.cookies = []*http.Cookie{{Name: "session", Value: "abc"}}

		r, _ := d.buildHAR(t, graphqlRequest(), true)

		got, _ := harHeader(r, "Cookie")
		assert.Equal(t, "session=abc", got)
	})

	t.Run("graphql_auth_and_unresolved_url", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := graphqlRequest()
		req.URL = "{{gql}}/graphql"
		req.AuthType = entities.AuthTypeAPIKey
		req.AuthData = `{"key":"api_key","value":"{{key}}","addTo":"query"}`

		r, _ := d.buildHAR(t, req, false)

		assert.Equal(t, "{{gql}}/graphql", r.URL)
		assert.Equal(t, []har.NameValue{{Name: "api_key", Value: "{{key}}"}}, r.QueryString)
	})

	t.Run("graphql_digest_warns", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := graphqlRequest()
		req.AuthType = entities.AuthTypeDigest
		req.AuthData = `{"username":"u","password":"p"}`

		r, warnings := d.buildHAR(t, req, true)

		_, hasAuth := harHeader(r, "Authorization")
		assert.False(t, hasAuth)
		assert.Empty(t, r.AuthNote)
		assert.True(t, warningWith(warnings, "HTTP only"), "warnings: %v", warnings)
	})
}

func TestBuildSnippetInput_GRPC(t *testing.T) {
	t.Run("grpc_fields_substituted", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"grpcHost": "localhost:50051", "name": "neo", "tok": "t1"})
		req := snippetRequest(entities.ProtocolGRPC, entities.MethodPOST, "{{grpcHost}}")
		req.Body = `{"name":"{{name}}"}`
		req.GRPCService = "helloworld.Greeter"
		req.GRPCMethod = "SayHello"
		req.GRPCMetadata = map[string][]string{
			"authorization": {"Bearer {{tok}}"},
			"x-trace":       {"{{missing}}"},
		}

		in := d.build(t, req, true)

		assert.Equal(t, entities.ProtocolGRPC, in.Protocol)
		assert.Nil(t, in.HAR)
		assert.Nil(t, in.WS)
		require.NotNil(t, in.GRPC)
		assert.Equal(t, request.GRPCSnippet{
			Target:  "localhost:50051",
			Service: "helloworld.Greeter",
			Method:  "SayHello",
			Message: `{"name":"neo"}`,
			Metadata: map[string][]string{
				"authorization": {"Bearer t1"},
				"x-trace":       {"{{missing}}"},
			},
		}, *in.GRPC)
		assert.Equal(t, []string{"Bearer {{tok}}"}, req.GRPCMetadata["authorization"])
	})
}

func TestBuildSnippetInput_WebSocket(t *testing.T) {
	t.Run("websocket_messages_and_subprotocols", func(t *testing.T) {
		d := newSnippetDeps(map[string]string{"host": "echo.example.com", "user": "neo"})
		req := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://{{host}}/ws")
		req.Headers = []entities.HeaderItem{{Key: "X-Client", Value: "{{client}}", Enabled: true}}
		req.AuthType = entities.AuthTypeBearer
		req.AuthData = `{"token":"{{token}}"}`
		req.Body = `{"version":1,"pingIntervalSec":0,"subprotocols":["graphql-ws"],"messages":[` +
			`{"id":"m1","name":"hello","format":"json","data":"{\"user\":\"{{user}}\"}"},` +
			`{"id":"m2","name":"raw","format":"binary","data":"e3t1c2VyfX0="}]}`

		in := d.build(t, req, true)

		assert.Equal(t, entities.ProtocolWebSocket, in.Protocol)
		assert.Nil(t, in.HAR)
		require.NotNil(t, in.WS)
		assert.Equal(t, "wss://echo.example.com/ws", in.WS.URL)
		assert.Equal(t, map[string][]string{
			"X-Client":      {"{{client}}"},
			"Authorization": {"Bearer {{token}}"},
		}, in.WS.Headers)
		assert.Equal(t, []string{"graphql-ws"}, in.WS.Subprotocols)
		assert.Equal(t, []request.WSSnippetMessage{
			{Name: "hello", Format: "json", Data: `{"user":"neo"}`},
			{Name: "raw", Format: "binary", Data: "e3t1c2VyfX0="},
		}, in.WS.Messages)
	})

	t.Run("websocket_query_auth_unresolved_host", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://{{host}}:{{port}}/ws")
		req.AuthType = entities.AuthTypeAPIKey
		req.AuthData = `{"key":"api_key","value":"{{key}}","addTo":"query"}`

		in := d.build(t, req, false)

		require.NotNil(t, in.WS)
		assert.Equal(t, "wss://{{host}}:{{port}}/ws?api_key={{key}}", in.WS.URL)
	})

	t.Run("websocket_port_then_reference_query_auth", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://{{host}}:{{port}}{{path}}")
		req.AuthType = entities.AuthTypeAPIKey
		req.AuthData = `{"key":"api_key","value":"{{key}}","addTo":"query"}`

		in := d.build(t, req, false)

		require.NotNil(t, in.WS)
		assert.Equal(t, "wss://{{host}}:{{port}}{{path}}?api_key={{key}}", in.WS.URL)
	})

	t.Run("websocket_inherited_query_auth", func(t *testing.T) {
		d := newSnippetDeps(nil)
		coll := d.collections.collections[testCollectionID]
		coll.AuthType = entities.AuthTypeAPIKey
		coll.AuthData = `{"key":"api_key","value":"{{key}}","addTo":"query"}`
		req := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://echo.example.com/ws")
		req.AuthType = entities.AuthTypeInherit

		in := d.build(t, req, false)

		require.NotNil(t, in.WS)
		assert.Equal(t, "wss://echo.example.com/ws?api_key={{key}}", in.WS.URL)
	})

	t.Run("websocket_digest_warns", func(t *testing.T) {
		d := newSnippetDeps(nil)
		req := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://echo.example.com/ws")
		req.AuthType = entities.AuthTypeAWSSigV4
		req.AuthData = `{"accessKeyId":"AK","secretAccessKey":"SK","region":"us-east-1","service":"execute-api"}`

		in := d.build(t, req, true)

		require.NotNil(t, in.WS)
		assert.Empty(t, in.WS.Headers)
		assert.True(t, warningWith(in.Warnings, "HTTP only"), "warnings: %v", in.Warnings)
	})
}

var snippetAuthProtocols = []struct {
	name string
	req  func() *entities.Request
}{
	{"http", func() *entities.Request {
		return snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
	}},
	{"graphql", func() *entities.Request {
		req := snippetRequest(entities.ProtocolGraphQL, entities.MethodPOST, "https://api.example.com/graphql")
		req.GraphQLQuery = "{ me { id } }"
		return req
	}},
	{"websocket", func() *entities.Request {
		return snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://api.example.com/ws")
	}},
}

func snippetAuthOf(t *testing.T, in request.SnippetInput) (string, map[string]string) {
	t.Helper()
	query := map[string]string{}
	switch {
	case in.HAR != nil:
		authz, _ := harHeader(in.HAR, "Authorization")
		for _, q := range in.HAR.QueryString {
			query[q.Name] = q.Value
		}
		return authz, query
	case in.WS != nil:
		var authz string
		if v := in.WS.Headers["Authorization"]; len(v) > 0 {
			authz = v[0]
		}
		_, rawQuery, _ := strings.Cut(in.WS.URL, "?")
		values, err := url.ParseQuery(rawQuery)
		require.NoError(t, err)
		for k, v := range values {
			query[k] = v[0]
		}
		return authz, query
	}
	t.Fatalf("no HAR or WS in %+v", in)
	return "", nil
}

func authPlaceholderWarnings(warnings []string) []string {
	var out []string
	for _, w := range warnings {
		if strings.HasPrefix(w, "Basic auth is not encoded") || strings.HasPrefix(w, "JWT is not signed") {
			out = append(out, w)
		}
	}
	return out
}

func TestBuildSnippetInput_AuthPlaceholders(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	const (
		basicWarning = "Basic auth is not encoded because these variables are not substituted: "
		jwtWarning   = "JWT is not signed because these variables are not substituted: "
		basicRefs    = `{"username":"{{user}}","password":"{{pass}}"}`
	)
	cases := []struct {
		name      string
		authType  entities.AuthType
		authData  string
		vars      map[string]string
		resolve   bool
		wantAuth  string
		wantQuery map[string]string
		warning   string
	}{
		{"basic_resolution_off", entities.AuthTypeBasic, basicRefs, nil, false,
			"Basic <base64 of {{user}}:{{pass}}>", nil, basicWarning + "{{user}}, {{pass}}"},
		{"basic_partly_resolved", entities.AuthTypeBasic, basicRefs, map[string]string{"user": "alice"}, true,
			"Basic <base64 of alice:{{pass}}>", nil, basicWarning + "{{pass}}"},
		{"basic_resolved", entities.AuthTypeBasic, basicRefs, map[string]string{"user": "alice", "pass": "pw"}, true,
			"Basic " + b64("alice:pw"), nil, ""},
		{"basic_unsafe_name", entities.AuthTypeBasic, `{"username":"{{a\"b}}","password":"p"}`, nil, false,
			`Basic <base64 of {{a"b}}:p>`, nil, basicWarning + `{{a"b}}`},
		{"basic_literal_resolution_off", entities.AuthTypeBasic, `{"username":"alice","password":"pw"}`, nil, false,
			"Basic " + b64("alice:pw"), nil, ""},
		{"jwt_hs256_resolution_off", entities.AuthTypeJWT, `{"alg":"HS256","secret":"{{secret}}"}`, nil, false,
			"Bearer <JWT signed with {{secret}}>", nil, jwtWarning + "{{secret}}"},
		{"jwt_rs256_resolution_off", entities.AuthTypeJWT, `{"alg":"RS256","privateKey":"{{pk}}"}`, nil, false,
			"Bearer <JWT signed with {{pk}}>", nil, jwtWarning + "{{pk}}"},
		{"jwt_rs256_missing_key", entities.AuthTypeJWT, `{"alg":"RS256","privateKey":"{{pk}}"}`, map[string]string{"other": "x"}, true,
			"Bearer <JWT signed with {{pk}}>", nil, jwtWarning + "{{pk}}"},
		{"jwt_claim_unresolved", entities.AuthTypeJWT, `{"alg":"HS256","secret":"{{secret}}","claims":{"sub":"{{sub}}"}}`, map[string]string{"secret": "s3cr3t"}, true,
			"Bearer <JWT>", nil, jwtWarning + "{{sub}}"},
		{"jwt_alg_unresolved", entities.AuthTypeJWT, `{"alg":"{{alg}}","secret":"{{secret}}"}`, nil, false,
			"Bearer <JWT signed with {{secret}}>", nil, jwtWarning + "{{alg}}, {{secret}}"},
		{"jwt_header_prefix", entities.AuthTypeJWT, `{"secret":"{{secret}}","headerPrefix":"JWT"}`, nil, false,
			"JWT <JWT signed with {{secret}}>", nil, jwtWarning + "{{secret}}"},
		{"jwt_empty_prefix", entities.AuthTypeJWT, `{"secret":"{{secret}}","headerPrefix":""}`, nil, false,
			"<JWT signed with {{secret}}>", nil, jwtWarning + "{{secret}}"},
		{"jwt_query", entities.AuthTypeJWT, `{"secret":"{{secret}}","addTo":"query"}`, nil, false,
			"", map[string]string{"token": "<JWT signed with {{secret}}>"}, jwtWarning + "{{secret}}"},
		{"jwt_query_param_reference", entities.AuthTypeJWT, `{"secret":"{{secret}}","addTo":"query","queryParam":"{{param}}"}`, nil, false,
			"", map[string]string{"{{param}}": "<JWT signed with {{secret}}>"}, jwtWarning + "{{secret}}"},
	}
	for _, p := range snippetAuthProtocols {
		for _, tc := range cases {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				d := newSnippetDeps(tc.vars)
				req := p.req()
				req.AuthType, req.AuthData = tc.authType, tc.authData

				in := d.build(t, req, tc.resolve)

				authz, query := snippetAuthOf(t, in)
				assert.Equal(t, tc.wantAuth, authz)
				if tc.wantQuery == nil {
					assert.Empty(t, query)
				} else {
					assert.Equal(t, tc.wantQuery, query)
				}
				if tc.warning == "" {
					assert.Empty(t, authPlaceholderWarnings(in.Warnings))
				} else {
					assert.Equal(t, []string{tc.warning}, authPlaceholderWarnings(in.Warnings))
				}
			})
		}
	}
}

func TestBuildSnippetInput_JWTSignedWhenResolved(t *testing.T) {
	cases := []struct {
		name     string
		authData string
		vars     map[string]string
		resolve  bool
		key      string
	}{
		{"all_references_resolved", `{"alg":"HS256","secret":"{{secret}}","claims":{"sub":"{{sub}}"}}`,
			map[string]string{"secret": "s3cr3t", "sub": "42"}, true, "s3cr3t"},
		{"unused_private_key_reference", `{"alg":"HS256","secret":"lit","privateKey":"{{pk}}","claims":{"sub":"42"}}`,
			nil, false, "lit"},
	}
	for _, p := range snippetAuthProtocols {
		for _, tc := range cases {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				d := newSnippetDeps(tc.vars)
				req := p.req()
				req.AuthType, req.AuthData = entities.AuthTypeJWT, tc.authData

				in := d.build(t, req, tc.resolve)

				authz, _ := snippetAuthOf(t, in)
				token, ok := strings.CutPrefix(authz, "Bearer ")
				require.True(t, ok, "Authorization: %q", authz)
				parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return []byte(tc.key), nil },
					jwt.WithValidMethods([]string{"HS256"}))
				require.NoError(t, err)
				assert.Equal(t, "42", parsed.Claims.(jwt.MapClaims)["sub"])
				assert.Empty(t, authPlaceholderWarnings(in.Warnings))
			})
		}
	}
}

func TestBuildSnippetInput_SecretVariables(t *testing.T) {
	vars := map[string]string{"host": "api.example.com", "user": "neo", "token": "sk-123", "pass": "pw-456"}
	secret := map[string]bool{"token": true, "pass": true}
	hidden := request.BuildSnippetOpt{ResolveVariables: true}
	included := request.BuildSnippetOpt{ResolveVariables: true, IncludeSecrets: true}
	newDeps := func() *snippetDeps {
		d := newSnippetDeps(vars)
		d.env.secret = secret
		return d
	}
	noSecretValues := func(t *testing.T, in request.SnippetInput) {
		t.Helper()
		raw, err := json.Marshal(in)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "sk-123")
		assert.NotContains(t, string(raw), "pw-456")
	}

	t.Run("http_fields_keep_secret_references", func(t *testing.T) {
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://{{host}}/u?t={{token}}")
		req.Headers = []entities.HeaderItem{{Key: "X-Token", Value: "{{token}}", Enabled: true}}
		req.BodyType = entities.BodyTypeJSON
		req.Body = `{"user":"{{user}}","token":"{{token}}"}`
		req.AuthType, req.AuthData = entities.AuthTypeBearer, `{"token":"{{token}}"}`

		in := newDeps().buildWith(t, req, hidden)

		require.NotNil(t, in.HAR)
		assert.Equal(t, "https://api.example.com/u", in.HAR.URL)
		assert.Equal(t, []har.NameValue{{Name: "t", Value: "{{token}}"}}, in.HAR.QueryString)
		got, _ := harHeader(in.HAR, "X-Token")
		assert.Equal(t, "{{token}}", got)
		got, _ = harHeader(in.HAR, "Authorization")
		assert.Equal(t, "Bearer {{token}}", got)
		require.NotNil(t, in.HAR.PostData)
		assert.Equal(t, `{"user":"neo","token":"{{token}}"}`, in.HAR.PostData.Text)
		noSecretValues(t, in)

		in = newDeps().buildWith(t, req, included)

		assert.Equal(t, []har.NameValue{{Name: "t", Value: "sk-123"}}, in.HAR.QueryString)
		got, _ = harHeader(in.HAR, "Authorization")
		assert.Equal(t, "Bearer sk-123", got)
		assert.Equal(t, `{"user":"neo","token":"sk-123"}`, in.HAR.PostData.Text)
	})

	t.Run("include_secrets_without_resolution", func(t *testing.T) {
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/u?t={{token}}")

		in := newDeps().buildWith(t, req, request.BuildSnippetOpt{IncludeSecrets: true})

		require.NotNil(t, in.HAR)
		assert.Equal(t, "https://{{host}}/u", in.HAR.URL)
		assert.Equal(t, []har.NameValue{{Name: "t", Value: "{{token}}"}}, in.HAR.QueryString)
	})

	t.Run("api_key_query", func(t *testing.T) {
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/u")
		req.AuthType, req.AuthData = entities.AuthTypeAPIKey, `{"key":"api_key","value":"{{token}}","addTo":"query"}`

		in := newDeps().buildWith(t, req, hidden)

		require.NotNil(t, in.HAR)
		assert.Equal(t, []har.NameValue{{Name: "api_key", Value: "{{token}}"}}, in.HAR.QueryString)
		noSecretValues(t, in)
	})

	t.Run("basic_and_jwt_become_placeholders", func(t *testing.T) {
		basic := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/u")
		basic.AuthType, basic.AuthData = entities.AuthTypeBasic, `{"username":"{{user}}","password":"{{pass}}"}`
		jwtReq := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/u")
		jwtReq.AuthType, jwtReq.AuthData = entities.AuthTypeJWT, `{"alg":"HS256","secret":"{{pass}}","claims":{"sub":"{{user}}"}}`

		b := newDeps().buildWith(t, basic, hidden)
		j := newDeps().buildWith(t, jwtReq, hidden)

		got, _ := harHeader(b.HAR, "Authorization")
		assert.Equal(t, "Basic <base64 of neo:{{pass}}>", got)
		got, _ = harHeader(j.HAR, "Authorization")
		assert.Equal(t, "Bearer <JWT signed with {{pass}}>", got)
		noSecretValues(t, b)
		noSecretValues(t, j)

		b = newDeps().buildWith(t, basic, included)
		j = newDeps().buildWith(t, jwtReq, included)

		got, _ = harHeader(b.HAR, "Authorization")
		assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("neo:pw-456")), got)
		got, _ = harHeader(j.HAR, "Authorization")
		token, _ := strings.CutPrefix(got, "Bearer ")
		_, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return []byte("pw-456"), nil },
			jwt.WithValidMethods([]string{"HS256"}))
		assert.NoError(t, err)
	})

	t.Run("graphql", func(t *testing.T) {
		req := snippetRequest(entities.ProtocolGraphQL, entities.MethodPOST, "https://{{host}}/graphql")
		req.GraphQLQuery = "query { me }"
		req.GraphQLVariables = `{"t":"{{token}}"}`

		in := newDeps().buildWith(t, req, hidden)

		require.NotNil(t, in.HAR)
		require.NotNil(t, in.HAR.PostData)
		assert.Equal(t, `{"query":"query { me }","variables":{"t":"{{token}}"}}`, in.HAR.PostData.Text)
		noSecretValues(t, in)
	})

	t.Run("grpc", func(t *testing.T) {
		req := snippetRequest(entities.ProtocolGRPC, entities.MethodPOST, "{{host}}:443")
		req.Body = `{"token":"{{token}}"}`
		req.GRPCMetadata = map[string][]string{"authorization": {"Bearer {{token}}"}}

		in := newDeps().buildWith(t, req, hidden)

		require.NotNil(t, in.GRPC)
		assert.Equal(t, "api.example.com:443", in.GRPC.Target)
		assert.Equal(t, `{"token":"{{token}}"}`, in.GRPC.Message)
		assert.Equal(t, map[string][]string{"authorization": {"Bearer {{token}}"}}, in.GRPC.Metadata)
		noSecretValues(t, in)
	})

	t.Run("websocket", func(t *testing.T) {
		req := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://{{host}}/ws?t={{token}}")
		req.Headers = []entities.HeaderItem{{Key: "X-Token", Value: "{{token}}", Enabled: true}}
		req.Body = `{"version":1,"messages":[{"id":"m1","name":"auth","format":"json","data":"{\"t\":\"{{token}}\"}"}]}`

		in := newDeps().buildWith(t, req, hidden)

		require.NotNil(t, in.WS)
		assert.Equal(t, "wss://api.example.com/ws?t={{token}}", in.WS.URL)
		assert.Equal(t, map[string][]string{"X-Token": {"{{token}}"}}, in.WS.Headers)
		require.Len(t, in.WS.Messages, 1)
		assert.Equal(t, `{"t":"{{token}}"}`, in.WS.Messages[0].Data)
		noSecretValues(t, in)
	})

	t.Run("oauth2_cache_keyed_by_secret_values", func(t *testing.T) {
		const authData = `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token","clientId":"cid","clientSecret":"{{pass}}"}`
		d := newDeps()
		store := newMemTokenStore()
		d.provider = auth.NewProvider(store, nil, nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/x")
		req.AuthType, req.AuthData = entities.AuthTypeOAuth2, authData
		resolved := strings.ReplaceAll(authData, "{{pass}}", "pw-456")
		store.seed(requestOwner(req.ID), configHashOf(t, resolved), "cached-at")

		in := d.buildWith(t, req, hidden)

		got, _ := harHeader(in.HAR, "Authorization")
		assert.Equal(t, "Bearer cached-at", got)
		noSecretValues(t, in)
	})

	t.Run("oauth2_placement_keeps_secret_references", func(t *testing.T) {
		const authData = `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token","clientId":"cid","addTo":"query","queryParam":"{{token}}"}`
		d := newDeps()
		store := newMemTokenStore()
		d.provider = auth.NewProvider(store, nil, nil)
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/x")
		req.AuthType, req.AuthData = entities.AuthTypeOAuth2, authData
		store.seed(requestOwner(req.ID), configHashOf(t, authData), "cached-at")

		in := d.buildWith(t, req, hidden)

		assert.Equal(t, []har.NameValue{{Name: "{{token}}", Value: "cached-at"}}, in.HAR.QueryString)
		noSecretValues(t, in)
	})

	t.Run("secret_anywhere_hides_the_name", func(t *testing.T) {
		d := newSnippetDeps(nil)
		d.env.active = []*entities.Variable{
			{Key: "token", Value: "sk-123", IsSecret: true, Enabled: true},
			{Key: "token", Value: "plain", Enabled: true},
		}
		req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/u?t={{token}}")

		in := d.buildWith(t, req, hidden)

		assert.Equal(t, []har.NameValue{{Name: "t", Value: "{{token}}"}}, in.HAR.QueryString)
	})
}

func TestBuildSnippetInput_HiddenSecretsWarning(t *testing.T) {
	const warning = "Secret values are hidden; turn on Include secret values to insert them"
	vars := map[string]string{"host": "api.example.com", "user": "neo", "token": "sk-123", "pass": "pw-456"}
	hidden := request.BuildSnippetOpt{ResolveVariables: true}
	newDeps := func() *snippetDeps {
		d := newSnippetDeps(vars)
		d.env.secret = map[string]bool{"token": true, "pass": true}
		return d
	}
	count := func(warnings []string) int {
		n := 0
		for _, w := range warnings {
			if w == warning {
				n++
			}
		}
		return n
	}

	httpReq := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://{{host}}/u?t={{token}}")
	httpReq.Headers = []entities.HeaderItem{{Key: "X-Pass", Value: "{{pass}}", Enabled: true}}
	httpReq.AuthType, httpReq.AuthData = entities.AuthTypeBasic, `{"username":"{{user}}","password":"{{pass}}"}`

	grpcReq := snippetRequest(entities.ProtocolGRPC, entities.MethodPOST, "{{host}}:443")
	grpcReq.GRPCMetadata = map[string][]string{"authorization": {"Bearer {{token}}"}}

	wsReq := snippetRequest(entities.ProtocolWebSocket, entities.MethodGET, "wss://{{host}}/ws")
	wsReq.Body = `{"version":1,"messages":[{"id":"m1","name":"auth","format":"json","data":"{\"t\":\"{{token}}\"}"}]}`

	gqlReq := snippetRequest(entities.ProtocolGraphQL, entities.MethodPOST, "https://{{host}}/graphql")
	gqlReq.GraphQLQuery, gqlReq.GraphQLVariables = "query { me }", `{"t":"{{token}}"}`

	t.Run("once_when_a_printed_secret_stays_a_reference", func(t *testing.T) {
		for _, req := range []*entities.Request{httpReq, grpcReq, wsReq, gqlReq} {
			in := newDeps().buildWith(t, req, hidden)
			assert.Equal(t, 1, count(in.Warnings), "%s: %v", req.Protocol, in.Warnings)
		}
	})

	t.Run("not_when_secrets_are_included_or_nothing_is_resolved", func(t *testing.T) {
		for _, opt := range []request.BuildSnippetOpt{
			{ResolveVariables: true, IncludeSecrets: true},
			{IncludeSecrets: true},
			{},
		} {
			for _, req := range []*entities.Request{httpReq, grpcReq, wsReq, gqlReq} {
				in := newDeps().buildWith(t, req, opt)
				assert.Zero(t, count(in.Warnings), "%+v %s: %v", opt, req.Protocol, in.Warnings)
			}
		}
	})

	t.Run("not_when_no_secret_is_printed", func(t *testing.T) {
		plain := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/u?u={{user}}&m={{missing}}")
		plain.Headers = []entities.HeaderItem{{Key: "X-Token", Value: "{{token}}", Enabled: false}}
		digest := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://{{host}}/u")
		digest.AuthType, digest.AuthData = entities.AuthTypeDigest, `{"username":"{{user}}","password":"{{pass}}"}`

		for _, req := range []*entities.Request{plain, digest} {
			in := newDeps().buildWith(t, req, hidden)
			assert.Zero(t, count(in.Warnings), "%v", in.Warnings)
		}
	})
}

func TestBuildSnippetInput_WarningsReadAsSentences(t *testing.T) {
	binary := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/upload")
	binary.BodyType, binary.Body = entities.BodyTypeBinary, "{{dir}}/a.bin"
	binary.Headers = []entities.HeaderItem{{Key: "X-Name", Value: "Жора", Enabled: true}}

	form := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/login")
	form.BodyType = entities.BodyTypeForm
	form.Body = `[{"key":"a","value":"1","type":"text","enabled":true},{"key":"a","value":"2","type":"text","enabled":true}]`

	oauth := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
	oauth.AuthType, oauth.AuthData = entities.AuthTypeOAuth2, oauth2SnippetAuthData

	basic := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
	basic.AuthType, basic.AuthData = entities.AuthTypeBasic, `{"username":"{{user}}","password":"p"}`

	gql := snippetRequest(entities.ProtocolGraphQL, entities.MethodPOST, "https://api.example.com/graphql")
	gql.GraphQLQuery, gql.GraphQLVariables = "{ me { id } }", `{"id": {{userId}}}`
	gql.AuthType, gql.AuthData = entities.AuthTypeDigest, `{"username":"u","password":"p"}`

	d := newSnippetDeps(nil)
	d.scripts.pre = `pm.environment.set("a", "1")`
	var warnings []string
	for _, req := range []*entities.Request{binary, form, oauth, basic, gql} {
		warnings = append(warnings, d.build(t, req, true).Warnings...)
	}

	require.Len(t, warnings, 12, "warnings: %v", warnings)
	for _, w := range warnings {
		first, _ := utf8.DecodeRuneInString(w)
		assert.True(t, unicode.IsUpper(first), "warning starts in lower case: %q", w)
	}
}

func TestBuildSnippetInput_DigestAndSigV4OnlyNoteTheCode(t *testing.T) {
	for _, authType := range []entities.AuthType{entities.AuthTypeDigest, entities.AuthTypeAWSSigV4} {
		t.Run(string(authType), func(t *testing.T) {
			req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
			req.AuthType, req.AuthData = authType, `{"username":"u","password":"p","accessKey":"a","secretKey":"s","region":"eu","service":"s3"}`

			r, warnings := newSnippetDeps(nil).buildHAR(t, req, true)

			assert.Equal(t, string(authType), r.AuthNote)
			assert.Empty(t, warnings)
		})
	}
}
