package request

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const (
	jwtDefaultAlg        = "HS256"
	jwtDefaultExpiresIn  = 3600
	jwtDefaultQueryParam = "token"
)

// jwtHeaderAllowlist are the JOSE header keys a user may set.
var jwtHeaderAllowlist = map[string]bool{
	"kid": true, "typ": true, "cty": true, "x5t": true, "x5u": true, "jku": true,
}

// The library signs with Token.Method but serialises the mutable header map, so a
// user-supplied alg (or a crit/b64 that changes how the token is read) would lie.
var jwtProtectedHeaderKeys = map[string]bool{"alg": true, "crit": true, "b64": true}

func jwtError(format string, args ...any) error {
	return &domain.ValidationError{Fields: map[string]string{"auth": fmt.Sprintf(format, args...)}}
}

// applyJWT signs a token into the Authorization header or the query, naming the query keys it injected.
func applyJWT(f auth.Fields, headers map[string][]string, rawURL string, now time.Time) (map[string][]string, string, []string, error) {
	token, err := signJWT(f, now)
	if err != nil {
		return headers, rawURL, nil, err
	}

	if f.Str("addTo") == "query" {
		param := f.Str("queryParam")
		if param == "" {
			param = jwtDefaultQueryParam
		}
		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil {
			return headers, rawURL, nil, fmt.Errorf("invalid URL for JWT: %w", parseErr)
		}
		q := parsed.Query()
		q.Set(param, token)
		parsed.RawQuery = q.Encode()
		return headers, parsed.String(), []string{param}, nil
	}

	prefix := "Bearer"
	if _, ok := f["headerPrefix"]; ok {
		prefix = f.Str("headerPrefix")
	}
	if prefix == "" {
		headers["Authorization"] = []string{token}
	} else {
		headers["Authorization"] = []string{prefix + " " + token}
	}
	return headers, rawURL, nil, nil
}

// signJWT builds and signs the token; now feeds iat/exp.
func signJWT(f auth.Fields, now time.Time) (string, error) {
	alg := f.Str("alg")
	if alg == "" {
		alg = jwtDefaultAlg
	}
	method := jwt.GetSigningMethod(alg)
	if method == nil {
		return "", jwtError("unknown JWT algorithm %q", alg)
	}
	key, err := jwtSigningKey(method, f)
	if err != nil {
		return "", err
	}
	claims, err := jwtClaims(f, now)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(method, claims)
	if err := applyJWTHeader(token, f); err != nil {
		return "", err
	}

	signed, err := token.SignedString(key)
	if err != nil {
		return "", jwtError("cannot sign the JWT: %v", err)
	}
	return signed, nil
}

func jwtSigningKey(method jwt.SigningMethod, f auth.Fields) (any, error) {
	switch method.(type) {
	case *jwt.SigningMethodHMAC:
		secret := f.Str("secret")
		if secret == "" {
			return nil, jwtError("JWT secret is required for %s", method.Alg())
		}
		if jwtBool(f, "secretBase64") {
			raw, err := base64.StdEncoding.DecodeString(secret)
			if err != nil {
				return nil, jwtError("JWT secret is not valid base64")
			}
			return raw, nil
		}
		return []byte(secret), nil

	case *jwt.SigningMethodRSA, *jwt.SigningMethodRSAPSS:
		pemKey, err := jwtPrivateKeyPEM(f, method)
		if err != nil {
			return nil, err
		}
		key, err := jwt.ParseRSAPrivateKeyFromPEM(pemKey)
		if err != nil {
			return nil, jwtError("JWT private key is not a valid RSA PEM key: %v", err)
		}
		return key, nil

	case *jwt.SigningMethodECDSA:
		pemKey, err := jwtPrivateKeyPEM(f, method)
		if err != nil {
			return nil, err
		}
		key, err := jwt.ParseECPrivateKeyFromPEM(pemKey)
		if err != nil {
			return nil, jwtError("JWT private key is not a valid EC PEM key: %v", err)
		}
		return key, nil

	case *jwt.SigningMethodEd25519:
		pemKey, err := jwtPrivateKeyPEM(f, method)
		if err != nil {
			return nil, err
		}
		key, err := jwt.ParseEdPrivateKeyFromPEM(pemKey)
		if err != nil {
			return nil, jwtError("JWT private key is not a valid Ed25519 PEM key: %v", err)
		}
		return key, nil

	default:
		return nil, jwtError("JWT algorithm %q is not supported", method.Alg())
	}
}

func jwtPrivateKeyPEM(f auth.Fields, method jwt.SigningMethod) ([]byte, error) {
	pemKey := f.Str("privateKey")
	if pemKey == "" {
		return nil, jwtError("JWT private key is required for %s", method.Alg())
	}
	return []byte(pemKey), nil
}

func jwtClaims(f auth.Fields, now time.Time) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	if raw, ok := f["claims"]; ok && raw != nil {
		obj, isObj := raw.(map[string]any)
		if !isObj {
			return nil, jwtError("JWT claims must be a JSON object")
		}
		for k, v := range obj {
			claims[k] = v
		}
	}

	if _, ok := claims["iat"]; !ok {
		claims["iat"] = now.Unix()
	}
	if _, ok := claims["exp"]; !ok {
		ttl, err := jwtExpiresIn(f)
		if err != nil {
			return nil, err
		}
		// A non-positive lifetime is how the user asks for a token without exp.
		if ttl > 0 {
			claims["exp"] = now.Add(time.Duration(ttl) * time.Second).Unix()
		}
	}
	return claims, nil
}

func jwtExpiresIn(f auth.Fields) (int64, error) {
	switch v := f["expiresIn"].(type) {
	case nil:
		return jwtDefaultExpiresIn, nil
	case float64:
		return int64(v), nil
	case string:
		if v == "" {
			return jwtDefaultExpiresIn, nil
		}
		seconds, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, jwtError("JWT expiresIn must be a number of seconds, got %q", v)
		}
		return seconds, nil
	default:
		return 0, jwtError("JWT expiresIn must be a number of seconds")
	}
}

func applyJWTHeader(token *jwt.Token, f auth.Fields) error {
	raw, ok := f["header"]
	if !ok || raw == nil {
		return nil
	}
	obj, isObj := raw.(map[string]any)
	if !isObj {
		return jwtError("JWT header must be a JSON object")
	}
	for k, v := range obj {
		if jwtProtectedHeaderKeys[k] {
			return jwtError("JWT header key %q cannot be set", k)
		}
		if !jwtHeaderAllowlist[k] {
			continue
		}
		token.Header[k] = v
	}
	return nil
}

// jwtBool reads a flag the frontend may store as a string or a JSON boolean.
func jwtBool(f auth.Fields, key string) bool {
	switch v := f[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}
