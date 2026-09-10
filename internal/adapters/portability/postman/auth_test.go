package postman_test

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func authKVs(pairs map[string]string) []postman.PostmanAuthKV {
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	kvs := make([]postman.PostmanAuthKV, 0, len(keys))
	for _, k := range keys {
		raw, _ := json.Marshal(pairs[k])
		kvs = append(kvs, postman.PostmanAuthKV{Key: k, Value: raw, Type: "string"})
	}
	return kvs
}

func authData(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	return m
}

func createdByName(created []request.Create) map[string]request.Create {
	byName := make(map[string]request.Create, len(created))
	for _, c := range created {
		byName[c.Name] = c
	}
	return byName
}

func TestImportCollection_AuthSchemesFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/oauth2-jwt.postman_collection.json")
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}
	result, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	require.Len(t, collUC.created, 1)
	assert.Equal(t, entities.AuthTypeOAuth2, collUC.created[0].AuthType)
	assert.Equal(t, map[string]any{
		"grant":        "client_credentials",
		"tokenUrl":     "https://idp.example.com/oauth2/token",
		"clientId":     "{{client_id}}",
		"clientSecret": "{{client_secret}}",
		"scope":        "read:orders write:orders",
		"clientAuth":   "body",
		"addTo":        "header",
	}, authData(t, collUC.created[0].AuthData))

	byName := createdByName(reqUC.created)
	require.Len(t, byName, 5)

	jwtReq := byName["Signed report"]
	assert.Equal(t, entities.AuthTypeJWT, jwtReq.AuthType)
	assert.Equal(t, map[string]any{
		"alg":          "HS256",
		"secret":       "a-string-secret-at-least-256-bits-long",
		"secretBase64": "false",
		"claims":       map[string]any{"sub": "1234567890", "role": "reporting"},
		"header":       map[string]any{"kid": "2026-09"},
		"headerPrefix": "Bearer",
		"queryParam":   "token",
		"addTo":        "header",
	}, authData(t, jwtReq.AuthData))

	awsReq := byName["Bucket upload"]
	assert.Equal(t, entities.AuthTypeAWSSigV4, awsReq.AuthType)
	assert.Equal(t, map[string]any{
		"accessKeyId":     "AKIDEXAMPLE",
		"secretAccessKey": "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"sessionToken":    "FQoDYXdzEJr",
		"region":          "us-east-1",
		"service":         "s3",
	}, authData(t, awsReq.AuthData))

	digestReq := byName["Digest resource"]
	assert.Equal(t, entities.AuthTypeDigest, digestReq.AuthType)
	assert.Equal(t, map[string]any{
		"username": "Mufasa",
		"password": "Circle Of Life",
	}, authData(t, digestReq.AuthData))

	assert.Equal(t, entities.AuthTypeNone, byName["Legacy service"].AuthType)

	pasted := byName["Pasted token"]
	assert.Equal(t, entities.AuthTypeBearer, pasted.AuthType)
	assert.Equal(t, map[string]any{"token": "ya29.a0AfB_pasted"}, authData(t, pasted.AuthData))

	require.Len(t, result.Warnings, 2)
	assert.Contains(t, result.Warnings, `request "Legacy service": auth type "ntlm" is not supported and was imported as no auth`)
	assert.Contains(t, result.Warnings,
		`request "Pasted token": the OAuth 2.0 block has an access token but no token URL; imported as a bearer token`)
}

func TestImportCollection_UnsupportedGrantWarning(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Grants", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{{
			Name: "Implicit",
			Request: &postman.PostmanRequest{
				Method: "GET",
				URL:    postman.PostmanURL{Raw: "https://api.example.com/x"},
				Auth: &postman.PostmanAuth{Type: "oauth2", OAuth2: authKVs(map[string]string{
					"grant_type":     "implicit",
					"accessTokenUrl": "https://idp.example.com/token",
					"authUrl":        "https://idp.example.com/authorize",
					"clientId":       "cid",
				})},
			},
		}},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)

	reqUC := &stubRequestUC{}
	result, err := postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC)
	require.NoError(t, err)

	require.Len(t, reqUC.created, 1)
	assert.Equal(t, entities.AuthTypeOAuth2, reqUC.created[0].AuthType)
	assert.Equal(t, "implicit", authData(t, reqUC.created[0].AuthData)["grant"])
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], `OAuth 2.0 grant "implicit" is not supported`)
}

func TestImportCollection_JWTPayloadNotAnObject(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "JWT", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{{
			Name: "Broken payload",
			Request: &postman.PostmanRequest{
				Method: "GET",
				URL:    postman.PostmanURL{Raw: "https://api.example.com/x"},
				Auth: &postman.PostmanAuth{Type: "jwt", JWT: authKVs(map[string]string{
					"algorithm": "HS256",
					"secret":    "a-string-secret-at-least-256-bits-long",
					"payload":   "not json",
				})},
			},
		}},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)

	reqUC := &stubRequestUC{}
	result, err := postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC)
	require.NoError(t, err)

	require.Len(t, reqUC.created, 1)
	assert.NotContains(t, authData(t, reqUC.created[0].AuthData), "claims")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "the JWT payload is not a JSON object")
}

func TestAuthRoundTrip(t *testing.T) {
	const fakePEM = "-----BEGIN RSA PRIVATE KEY-----\nZmFrZS10ZXN0LWtleQ==\n-----END RSA PRIVATE KEY-----"

	cases := []struct {
		name     string
		authType entities.AuthType
		data     string
	}{
		{
			name:     "oauth2 password grant",
			authType: entities.AuthTypeOAuth2,
			data: `{"grant":"password","tokenUrl":"https://idp.example.com/token","authUrl":"https://idp.example.com/authorize",` +
				`"clientId":"cid","clientSecret":"csecret","scope":"openid","audience":"https://api.example.com",` +
				`"username":"alice","password":"s3cret","clientAuth":"basic","addTo":"query","headerPrefix":"Bearer",` +
				`"queryParam":"my_token"}`,
		},
		{
			name:     "oauth2 public client",
			authType: entities.AuthTypeOAuth2,
			data: `{"grant":"authorization_code","tokenUrl":"https://idp.example.com/token",` +
				`"authUrl":"https://idp.example.com/authorize","deviceAuthUrl":"https://idp.example.com/device",` +
				`"clientId":"cid","clientAuth":"none","redirectPort":"31234"}`,
		},
		{
			name:     "oauth2 device code",
			authType: entities.AuthTypeOAuth2,
			data: `{"grant":"device_code","tokenUrl":"https://idp.example.com/token",` +
				`"deviceAuthUrl":"https://idp.example.com/device","clientId":"cid","clientAuth":"body","clientSecret":"cs"}`,
		},
		{
			name:     "jwt rs256",
			authType: entities.AuthTypeJWT,
			data: `{"alg":"RS256","privateKey":"` + jsonEscaped(fakePEM) + `","claims":{"sub":"42","scope":"read"},` +
				`"header":{"kid":"k1"},"secretBase64":"false","headerPrefix":"JWT","queryParam":"jwt","addTo":"query",` +
				`"expiresIn":"60"}`,
		},
		{
			name:     "digest",
			authType: entities.AuthTypeDigest,
			data:     `{"username":"Mufasa","password":"Circle Of Life"}`,
		},
		{
			name:     "aws sigv4",
			authType: entities.AuthTypeAWSSigV4,
			data: `{"accessKeyId":"AKIDEXAMPLE","secretAccessKey":"wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",` +
				`"sessionToken":"FQoDYXdzEJr","region":"eu-west-1","service":"execute-api"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootID := uuid.New()
			exported, err := postman.ExportCollection(rootID,
				[]*entities.Collection{{ID: rootID, Name: "Root", AuthType: tc.authType, AuthData: tc.data}},
				[]*entities.Request{{
					ID: uuid.New(), CollectionID: rootID, Name: "Req", Protocol: entities.ProtocolHTTP,
					Method: entities.MethodGET, URL: "https://api.example.com/x",
					BodyType: entities.BodyTypeNone, AuthType: tc.authType, AuthData: tc.data,
				}})
			require.NoError(t, err)

			collUC := &stubCollectionUC{}
			reqUC := &stubRequestUC{}
			result, err := postman.ImportCollection(context.Background(), exported, postman.ImportOpts{
				WorkspaceID: uuid.New(), UserID: "local_user",
			}, collUC, reqUC)
			require.NoError(t, err)
			assert.Empty(t, result.Warnings)

			require.Len(t, collUC.created, 1)
			require.Len(t, reqUC.created, 1)
			assert.Equal(t, tc.authType, collUC.created[0].AuthType)
			assert.Equal(t, authData(t, tc.data), authData(t, collUC.created[0].AuthData))
			assert.Equal(t, tc.authType, reqUC.created[0].AuthType)
			assert.Equal(t, authData(t, tc.data), authData(t, reqUC.created[0].AuthData))
		})
	}
}

func TestExportCollection_JWTSecretBase64IsJSONBoolean(t *testing.T) {
	rootID := uuid.New()
	exported, err := postman.ExportCollection(rootID,
		[]*entities.Collection{{
			ID: rootID, Name: "Root", AuthType: entities.AuthTypeJWT,
			AuthData: `{"alg":"HS256","secret":"a-string-secret-at-least-256-bits-long","secretBase64":"false"}`,
		}}, nil)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(exported, &pc))
	require.NotNil(t, pc.Auth)

	for _, kv := range pc.Auth.JWT {
		if kv.Key == "isSecretBase64Encoded" {
			assert.Equal(t, "false", string(kv.Value))
			return
		}
	}
	t.Fatal("isSecretBase64Encoded was not exported")
}

func TestExportCollection_NumericJWTLifetime(t *testing.T) {
	rootID := uuid.New()
	exported, err := postman.ExportCollection(rootID,
		[]*entities.Collection{{
			ID: rootID, Name: "Root", AuthType: entities.AuthTypeJWT,
			AuthData: `{"alg":"HS256","secret":"a-string-secret-at-least-256-bits-long","expiresIn":60}`,
		}}, nil)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(exported, &pc))
	require.NotNil(t, pc.Auth)

	for _, kv := range pc.Auth.JWT {
		if kv.Key == "tetivaExpiresIn" {
			assert.JSONEq(t, `"60"`, string(kv.Value))
			return
		}
	}
	t.Fatal("a numeric expiresIn was not exported")
}

func jsonEscaped(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw[1 : len(raw)-1])
}

// genericItem reads the fixture without our DTO, so a change to the DTO cannot
// hide a change in what headers and form fields we import.
type genericItem struct {
	Name    string        `json:"name"`
	Item    []genericItem `json:"item"`
	Request *struct {
		Header []map[string]any `json:"header"`
		URL    struct {
			Raw string `json:"raw"`
		} `json:"url"`
		Body *struct {
			Mode     string           `json:"mode"`
			FormData []map[string]any `json:"formdata"`
		} `json:"body"`
	} `json:"request"`
}

func TestImportCollection_RealFileKeyValuesUnchanged(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.postman_collection.json")
	if os.IsNotExist(err) {
		t.Skip("testdata/sample.postman_collection.json not found")
	}
	require.NoError(t, err)

	var generic struct {
		Item []genericItem `json:"item"`
	}
	require.NoError(t, json.Unmarshal(data, &generic))

	var wantURLs []string
	var wantHeaders, wantForms [][][3]string
	var walk func(items []genericItem)
	walk = func(items []genericItem) {
		for _, it := range items {
			switch {
			case it.Item != nil:
				walk(it.Item)
			case it.Request != nil:
				wantURLs = append(wantURLs, it.Request.URL.Raw)
				headers := [][3]string{}
				for _, h := range it.Request.Header {
					headers = append(headers, [3]string{str(h["key"]), str(h["value"]), boolStr(h["disabled"] == true)})
				}
				wantHeaders = append(wantHeaders, headers)

				form := [][3]string{}
				if it.Request.Body != nil && it.Request.Body.Mode == "formdata" {
					for _, f := range it.Request.Body.FormData {
						fieldType := str(f["type"])
						if fieldType == "" {
							fieldType = "text"
						}
						form = append(form, [3]string{str(f["key"]), str(f["value"]), fieldType})
					}
				}
				wantForms = append(wantForms, form)
			}
		}
	}
	walk(generic.Item)

	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC)
	require.NoError(t, err)
	require.Len(t, reqUC.created, len(wantHeaders))

	for i, created := range reqUC.created {
		require.Equal(t, wantURLs[i], created.URL, "request %d is not the one we walked to", i)

		got := [][3]string{}
		for _, h := range created.Headers {
			got = append(got, [3]string{h.Key, h.Value, boolStr(!h.Enabled)})
		}
		assert.Equal(t, wantHeaders[i], got, "headers of %q", created.Name)

		gotForm := [][3]string{}
		if created.BodyType == entities.BodyTypeForm {
			var fields []map[string]any
			require.NoError(t, json.Unmarshal([]byte(created.Body), &fields))
			for _, f := range fields {
				gotForm = append(gotForm, [3]string{str(f["key"]), str(f["value"]), str(f["type"])})
			}
		}
		assert.Equal(t, wantForms[i], gotForm, "form fields of %q", created.Name)
	}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func boolStr(b bool) string {
	if b {
		return "disabled"
	}
	return "enabled"
}
