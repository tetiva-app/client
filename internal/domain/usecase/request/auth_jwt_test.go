package request

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

var jwtTestNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

// parseSignedJWT verifies the signature with key and returns the decoded token;
// now is the clock the expiry is validated against.
func parseSignedJWT(t *testing.T, token string, key any, now time.Time) *jwt.Token {
	t.Helper()
	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	return parsed
}

func jwtClaimNumber(t *testing.T, tok *jwt.Token, key string) int64 {
	t.Helper()
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims: got %T", tok.Claims)
	}
	num, ok := claims[key].(float64)
	if !ok {
		t.Fatalf("claim %q: got %#v", key, claims[key])
	}
	return int64(num)
}

func authFieldsError(t *testing.T, err error) *domain.ValidationError {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected a ValidationError, got %v", err)
	}
	if ve.Fields["auth"] == "" {
		t.Fatalf("expected an \"auth\" field, got %v", ve.Fields)
	}
	return ve
}

func TestSignJWT_HS256(t *testing.T) {
	token, err := signJWT(mustFields(t, `{"alg":"HS256","secret":"s3cr3t","claims":{"sub":"1234567890","admin":true}}`), jwtTestNow)
	if err != nil {
		t.Fatalf("signJWT: %v", err)
	}

	parsed := parseSignedJWT(t, token, []byte("s3cr3t"), jwtTestNow)
	if parsed.Method.Alg() != "HS256" {
		t.Errorf("alg: got %q", parsed.Method.Alg())
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "1234567890" || claims["admin"] != true {
		t.Errorf("claims not carried through: %v", claims)
	}
	if got := jwtClaimNumber(t, parsed, "iat"); got != jwtTestNow.Unix() {
		t.Errorf("iat: got %d, want %d", got, jwtTestNow.Unix())
	}
	if got := jwtClaimNumber(t, parsed, "exp"); got != jwtTestNow.Add(time.Hour).Unix() {
		t.Errorf("exp: got %d, want %d", got, jwtTestNow.Add(time.Hour).Unix())
	}
}

func TestSignJWT_ExpiresInAndExplicitClaims(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantIat  int64
		wantExp  int64
		noExpiry bool
	}{
		{
			name:    "custom expiresIn",
			raw:     `{"secret":"s3cr3t","expiresIn":"60"}`,
			wantIat: jwtTestNow.Unix(),
			wantExp: jwtTestNow.Add(time.Minute).Unix(),
		},
		{
			name:    "numeric expiresIn",
			raw:     `{"secret":"s3cr3t","expiresIn":120}`,
			wantIat: jwtTestNow.Unix(),
			wantExp: jwtTestNow.Add(2 * time.Minute).Unix(),
		},
		{
			name:    "claims win over the clock",
			raw:     `{"secret":"s3cr3t","claims":{"iat":100,"exp":4102444800}}`,
			wantIat: 100,
			wantExp: 4102444800,
		},
		{
			name:     "zero expiresIn omits exp",
			raw:      `{"secret":"s3cr3t","expiresIn":"0"}`,
			wantIat:  jwtTestNow.Unix(),
			noExpiry: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := signJWT(mustFields(t, tt.raw), jwtTestNow)
			if err != nil {
				t.Fatalf("signJWT: %v", err)
			}
			parsed := parseSignedJWT(t, token, []byte("s3cr3t"), jwtTestNow)
			if got := jwtClaimNumber(t, parsed, "iat"); got != tt.wantIat {
				t.Errorf("iat: got %d, want %d", got, tt.wantIat)
			}
			claims := parsed.Claims.(jwt.MapClaims)
			if tt.noExpiry {
				if _, ok := claims["exp"]; ok {
					t.Errorf("exp must be absent, got %v", claims["exp"])
				}
				return
			}
			if got := jwtClaimNumber(t, parsed, "exp"); got != tt.wantExp {
				t.Errorf("exp: got %d, want %d", got, tt.wantExp)
			}
		})
	}
}

func TestSignJWT_SecretBase64(t *testing.T) {
	secret := []byte{0x00, 0x01, 0xfe, 0xff, 'k', 'e', 'y'}
	encoded := base64.StdEncoding.EncodeToString(secret)

	token, err := signJWT(mustFields(t, `{"secret":"`+encoded+`","secretBase64":"true"}`), jwtTestNow)
	if err != nil {
		t.Fatalf("signJWT: %v", err)
	}
	parseSignedJWT(t, token, secret, jwtTestNow)

	if _, err := signJWT(mustFields(t, `{"secret":"not base64 !!","secretBase64":"true"}`), jwtTestNow); err == nil {
		t.Fatal("expected an error for an undecodable base64 secret")
	} else {
		authFieldsError(t, err)
	}
}

func TestSignJWT_RS256(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	fields := mustFields(t, `{"alg":"RS256","claims":{"sub":"rs"}}`)
	fields["privateKey"] = string(pemKey)

	token, err := signJWT(fields, jwtTestNow)
	if err != nil {
		t.Fatalf("signJWT: %v", err)
	}
	parsed := parseSignedJWT(t, token, &key.PublicKey, jwtTestNow)
	if parsed.Method.Alg() != "RS256" {
		t.Errorf("alg: got %q", parsed.Method.Alg())
	}
	if parsed.Claims.(jwt.MapClaims)["sub"] != "rs" {
		t.Errorf("claims: %v", parsed.Claims)
	}

	broken := mustFields(t, `{"alg":"RS256","privateKey":"-----BEGIN RSA PRIVATE KEY-----\nnope\n-----END RSA PRIVATE KEY-----"}`)
	if _, err := signJWT(broken, jwtTestNow); err == nil {
		t.Fatal("expected an error for an unparsable PEM key")
	} else {
		authFieldsError(t, err)
	}
}

func TestSignJWT_CustomHeaderKeys(t *testing.T) {
	token, err := signJWT(mustFields(t, `{"secret":"s3cr3t","header":{"kid":"key-1","typ":"at+jwt","nope":"ignored"}}`), jwtTestNow)
	if err != nil {
		t.Fatalf("signJWT: %v", err)
	}
	parsed := parseSignedJWT(t, token, []byte("s3cr3t"), jwtTestNow)
	if parsed.Header["kid"] != "key-1" {
		t.Errorf("kid: got %v", parsed.Header["kid"])
	}
	if parsed.Header["typ"] != "at+jwt" {
		t.Errorf("typ: got %v", parsed.Header["typ"])
	}
	if _, ok := parsed.Header["nope"]; ok {
		t.Errorf("header must only carry allow-listed keys: %v", parsed.Header)
	}
	if parsed.Header["alg"] != "HS256" {
		t.Errorf("alg: got %v", parsed.Header["alg"])
	}
}

// alg/crit/b64 decide how the token is verified; the library signs with
// Token.Method while serialising Header, so a user-supplied alg would lie.
func TestSignJWT_ProtectedHeaderKeysRejected(t *testing.T) {
	for _, key := range []string{"alg", "crit", "b64"} {
		_, err := signJWT(mustFields(t, `{"secret":"s3cr3t","header":{"`+key+`":"none"}}`), jwtTestNow)
		if err == nil {
			t.Fatalf("%q in the header must be rejected", key)
		}
		ve := authFieldsError(t, err)
		if !strings.Contains(ve.Fields["auth"], key) {
			t.Errorf("%q: message should name the key, got %q", key, ve.Fields["auth"])
		}
	}
}

func TestSignJWT_Errors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "unknown algorithm", raw: `{"alg":"HS999","secret":"s3cr3t"}`},
		{name: "none algorithm", raw: `{"alg":"none","secret":"s3cr3t"}`},
		{name: "missing secret", raw: `{"alg":"HS256"}`},
		{name: "missing private key", raw: `{"alg":"RS256"}`},
		{name: "claims not an object", raw: `{"secret":"s3cr3t","claims":"oops"}`},
		{name: "header not an object", raw: `{"secret":"s3cr3t","header":"oops"}`},
		{name: "expiresIn not a number", raw: `{"secret":"s3cr3t","expiresIn":"soon"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := signJWT(mustFields(t, tt.raw), jwtTestNow); err == nil {
				t.Fatal("expected an error")
			} else {
				authFieldsError(t, err)
			}
		})
	}
}

func TestApplyHeaderAuth_JWTHeader(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantPrefix string
	}{
		{name: "default prefix", raw: `{"secret":"s3cr3t"}`, wantPrefix: "Bearer "},
		{name: "custom prefix", raw: `{"secret":"s3cr3t","headerPrefix":"JWT"}`, wantPrefix: "JWT "},
		{name: "empty prefix", raw: `{"secret":"s3cr3t","headerPrefix":""}`, wantPrefix: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, rawURL, keys, err := applyHeaderAuth(entities.AuthTypeJWT, mustFields(t, tt.raw),
				map[string][]string{}, "https://api.example.com/data")
			if err != nil {
				t.Fatalf("applyHeaderAuth: %v", err)
			}
			got := headers["Authorization"]
			if len(got) != 1 || !strings.HasPrefix(got[0], tt.wantPrefix) {
				t.Fatalf("Authorization: got %v, want prefix %q", got, tt.wantPrefix)
			}
			parseSignedJWT(t, strings.TrimPrefix(got[0], tt.wantPrefix), []byte("s3cr3t"), time.Now())
			if rawURL != "https://api.example.com/data" || keys != nil {
				t.Errorf("URL must stay untouched: %q %v", rawURL, keys)
			}
		})
	}
}

func TestApplyHeaderAuth_JWTQuery(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		param string
	}{
		{name: "default param", raw: `{"secret":"s3cr3t","addTo":"query"}`, param: "token"},
		{name: "custom param", raw: `{"secret":"s3cr3t","addTo":"query","queryParam":"access_token"}`, param: "access_token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, rawURL, keys, err := applyHeaderAuth(entities.AuthTypeJWT, mustFields(t, tt.raw),
				map[string][]string{}, "https://api.example.com/data?page=2")
			if err != nil {
				t.Fatalf("applyHeaderAuth: %v", err)
			}
			if len(headers) != 0 {
				t.Errorf("expected no headers, got %v", headers)
			}
			parsed, err := url.Parse(rawURL)
			if err != nil {
				t.Fatalf("parse result URL: %v", err)
			}
			if parsed.Query().Get("page") != "2" {
				t.Errorf("existing query lost: %q", rawURL)
			}
			token := parsed.Query().Get(tt.param)
			if token == "" {
				t.Fatalf("%q missing from %q", tt.param, rawURL)
			}
			parseSignedJWT(t, token, []byte("s3cr3t"), time.Now())
			if len(keys) != 1 || keys[0] != tt.param {
				t.Errorf("query keys: got %v, want [%s]", keys, tt.param)
			}
		})
	}
}

func TestApplyHeaderAuth_JWTInvalidURL(t *testing.T) {
	_, _, _, err := applyHeaderAuth(entities.AuthTypeJWT,
		mustFields(t, `{"secret":"s3cr3t","addTo":"query"}`),
		map[string][]string{}, "://nope")
	if err == nil {
		t.Fatal("expected an error for an unparsable URL")
	}
}
