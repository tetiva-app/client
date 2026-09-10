package requester

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const sigv4Algorithm = "AWS4-HMAC-SHA256"

// sigv4Creds are the substituted aws_sigv4 auth fields.
type sigv4Creds struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Service         string
}

func sigv4CredsFromFields(f auth.Fields) sigv4Creds {
	return sigv4Creds{
		AccessKeyID:     f.Str("accessKeyId"),
		SecretAccessKey: f.Str("secretAccessKey"),
		SessionToken:    f.Str("sessionToken"),
		Region:          f.Str("region"),
		Service:         f.Str("service"),
	}
}

// Header signing: the signed set is built from the request — host, every x-amz-* header,
// plus content-type and content-md5 when present.
func signSigV4(req *http.Request, payloadHash string, c sigv4Creds, now time.Time) error {
	missing := map[string]string{}
	for field, value := range map[string]string{
		"accessKeyId": c.AccessKeyID, "secretAccessKey": c.SecretAccessKey,
		"region": c.Region, "service": c.Service,
	} {
		if strings.TrimSpace(value) == "" {
			missing[field] = "required for AWS Signature V4"
		}
	}
	if len(missing) > 0 {
		return &domain.ValidationError{Fields: missing}
	}

	if payloadHash == "" {
		payloadHash = emptyPayloadHash
	}
	utc := now.UTC()
	amzDate := utc.Format("20060102T150405Z")
	dateStamp := utc.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	if c.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", c.SessionToken)
	}
	// S3 verifies the payload hash header; other services ignore it, and signing
	// it there would diverge from the AWS test-suite vectors.
	if isS3Service(c.Service) || req.Header.Get("X-Amz-Content-Sha256") != "" {
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	names, canonicalHeaders := canonicalSigV4Headers(req.Header, host)
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalSigV4Path(req.URL, c.Service),
		canonicalSigV4Query(req.URL),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, c.Region, c.Service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		sigv4Algorithm,
		amzDate,
		scope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	key := hmacSHA256([]byte("AWS4"+c.SecretAccessKey), dateStamp)
	key = hmacSHA256(key, c.Region)
	key = hmacSHA256(key, c.Service)
	key = hmacSHA256(key, "aws4_request")

	req.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		sigv4Algorithm, c.AccessKeyID, scope, signedHeaders, hex.EncodeToString(hmacSHA256(key, stringToSign))))
	return nil
}

func isS3Service(service string) bool {
	return strings.HasPrefix(strings.ToLower(service), "s3")
}

// Authorization is never signed: it is what we are producing.
func canonicalSigV4Headers(h http.Header, host string) ([]string, string) {
	values := map[string][]string{"host": {host}}
	for name, vals := range h {
		lower := strings.ToLower(name)
		if lower == "authorization" {
			continue
		}
		if !strings.HasPrefix(lower, "x-amz-") && lower != "content-type" && lower != "content-md5" {
			continue
		}
		values[lower] = vals
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		cleaned := make([]string, 0, len(values[name]))
		for _, v := range values[name] {
			cleaned = append(cleaned, collapseSpaces(v))
		}
		b.WriteString(strings.Join(cleaned, ","))
		b.WriteByte('\n')
	}
	return names, b.String()
}

// canonicalSigV4Path escapes the path once for S3 (which signs object keys
// verbatim) and twice everywhere else, as AWS specifies.
func canonicalSigV4Path(u *url.URL, service string) string {
	path := u.EscapedPath()
	if path == "" {
		return "/"
	}
	if isS3Service(service) {
		return path
	}
	return awsEscape(path, true)
}

// Sorted by name then value and re-encoded with AWS rules (space is %20, not +). Reads
// RawQuery, not Query(): the server signs what is on the wire, where "+" is a literal plus.
func canonicalSigV4Query(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	type pair struct{ key, value string }
	var pairs []pair
	for _, raw := range strings.Split(u.RawQuery, "&") {
		if raw == "" {
			continue
		}
		key, value, _ := strings.Cut(raw, "=")
		pairs = append(pairs, pair{
			key:   awsEscape(unescapeRawQueryPart(key), false),
			value: awsEscape(unescapeRawQueryPart(value), false),
		})
	}
	if len(pairs) == 0 {
		return ""
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].value < pairs[j].value
	})

	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, p.key+"="+p.value)
	}
	return strings.Join(parts, "&")
}

// unescapeRawQueryPart decodes percent-escapes only; an invalid escape is
// signed exactly as the server receives it.
func unescapeRawQueryPart(s string) string {
	decoded, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}

// awsEscape percent-encodes everything outside the unreserved set; keepSlash is
// set for paths, where "/" stays a separator.
func awsEscape(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		case ch == '/' && keepSlash:
			b.WriteByte('/')
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

// collapseSpaces trims a header value and folds runs of spaces into one.
func collapseSpaces(v string) string {
	out := strings.TrimSpace(v)
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return out
}

func hexSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
