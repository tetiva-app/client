package requester

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain"
)

// Credentials, clock and the first three expected signatures come from the AWS
// SigV4 test suite; the rest were cross-checked against aws-sdk-go-v2's signer.
const (
	testAWSKeyID  = "AKIDEXAMPLE"
	testAWSSecret = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
)

var testAWSTime = time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)

func TestSignSigV4_Vectors(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		url     string
		headers map[string]string
		body    string
		creds   sigv4Creds
		want    string
		wantSHA string // expected x-amz-content-sha256 header, "" when it must be absent
	}{
		{
			name: "get-vanilla", method: http.MethodGet, url: "https://example.amazonaws.com/",
			creds: sigv4Creds{Region: "us-east-1", Service: "service"},
			want:  "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31",
		},
		{
			name: "post-vanilla-query", method: http.MethodPost, url: "https://example.amazonaws.com/?Param1=value1",
			creds: sigv4Creds{Region: "us-east-1", Service: "service"},
			want:  "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=28038455d6de14eafc1f9222cf5aa6f1a96197d7deb8263271d420d138af7f11",
		},
		{
			name: "post-x-www-form-urlencoded", method: http.MethodPost, url: "https://example.amazonaws.com/",
			headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			body:    "Param1=value1",
			creds:   sigv4Creds{Region: "us-east-1", Service: "service"},
			want:    "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=ff11897932ad3f4e8b18135d722051e5ac45fc38421b1da7b9d196a0fe09473a",
		},
		{
			name: "session token", method: http.MethodGet, url: "https://example.amazonaws.com/",
			creds: sigv4Creds{Region: "us-east-1", Service: "service", SessionToken: "AQoDYXdzEPT//////////wEXAMPLEtEXAMPLE"},
			want:  "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=edaa8d606468f4d6bcd2ed19b063100237fc7c92517ce3ef0bfb21177c74ac5d",
		},
		{
			name: "s3 PutObject with metadata", method: http.MethodPut, url: "https://examplebucket.s3.amazonaws.com/my-object.txt",
			headers: map[string]string{"Content-Type": "text/plain", "X-Amz-Meta-Foo": "bar"},
			body:    "hello world",
			creds:   sigv4Creds{Region: "us-east-1", Service: "s3"},
			want:    "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/s3/aws4_request, SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date;x-amz-meta-foo, Signature=3537ecd22b3e224f679f67aefb614e63baeff840c7e8b8906c582fa0db37da39",
			wantSHA: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name: "x-amz-target", method: http.MethodPost, url: "https://dynamodb.us-east-1.amazonaws.com/",
			headers: map[string]string{"Content-Type": "application/x-amz-json-1.0", "X-Amz-Target": "DynamoDB_20120810.ListTables"},
			body:    `{"Limit":10}`,
			creds:   sigv4Creds{Region: "us-east-1", Service: "dynamodb"},
			want:    "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/dynamodb/aws4_request, SignedHeaders=content-type;host;x-amz-date;x-amz-target, Signature=11041d969631ff3f1d1f8d3b5fac1fe918edd3847f40dcc4423df1f26416af70",
		},
		{
			name: "s3 path is escaped once", method: http.MethodPut, url: "https://examplebucket.s3.amazonaws.com/my%20object.txt",
			body:    "hello",
			creds:   sigv4Creds{Region: "us-east-1", Service: "s3"},
			want:    "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=761cc0c32c41b8c1eef51a5398fb1c8b36ccea18a2c6592b6bc81a1968b44adc",
			wantSHA: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			name: "non-s3 path is escaped twice", method: http.MethodGet, url: "https://example.amazonaws.com/my%20object",
			creds: sigv4Creds{Region: "us-east-1", Service: "service"},
			want:  "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=c61fac14909992917274d02f2be6b4fbeb731d6b6eaff9c506aa8c0819d0527f",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			sum := sha256.Sum256([]byte(tc.body))
			creds := tc.creds
			creds.AccessKeyID = testAWSKeyID
			creds.SecretAccessKey = testAWSSecret

			if signErr := signSigV4(req, hex.EncodeToString(sum[:]), creds, testAWSTime); signErr != nil {
				t.Fatalf("sign: %v", signErr)
			}
			if got := req.Header.Get("Authorization"); got != tc.want {
				t.Errorf("Authorization mismatch\n got: %s\nwant: %s", got, tc.want)
			}
			if got := req.Header.Get("X-Amz-Date"); got != "20150830T123600Z" {
				t.Errorf("expected X-Amz-Date, got %q", got)
			}
			if got := req.Header.Get("X-Amz-Content-Sha256"); got != tc.wantSHA {
				t.Errorf("expected x-amz-content-sha256 %q, got %q", tc.wantSHA, got)
			}
			if tc.creds.SessionToken != "" && req.Header.Get("X-Amz-Security-Token") != tc.creds.SessionToken {
				t.Errorf("session token not set: %q", req.Header.Get("X-Amz-Security-Token"))
			}
		})
	}
}

func TestSignSigV4_MissingFields(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.amazonaws.com/", nil)
	err := signSigV4(req, emptyPayloadHash, sigv4Creds{AccessKeyID: "AKID"}, testAWSTime)

	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	for _, field := range []string{"secretAccessKey", "region", "service"} {
		if ve.Fields[field] == "" {
			t.Errorf("expected %q to be reported missing, got %v", field, ve.Fields)
		}
	}
	if ve.Fields["accessKeyId"] != "" {
		t.Errorf("accessKeyId was provided, should not be reported: %v", ve.Fields)
	}
}

func TestSignSigV4_IgnoresExistingAuthorization(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.amazonaws.com/", nil)
	req.Header.Set("Authorization", "Bearer stale")
	req.Header.Set("Accept", "application/json")

	if err := signSigV4(req, emptyPayloadHash, sigv4Creds{
		AccessKeyID: testAWSKeyID, SecretAccessKey: testAWSSecret, Region: "us-east-1", Service: "service",
	}, testAWSTime); err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Unsigned headers must stay out of SignedHeaders, and the old Authorization
	// must be replaced rather than signed.
	if !strings.Contains(req.Header.Get("Authorization"), "SignedHeaders=host;x-amz-date,") {
		t.Fatalf("unexpected signed headers: %s", req.Header.Get("Authorization"))
	}
}

func TestCanonicalSigV4Query(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "https://example.com/", want: ""},
		{name: "sorted by name then value", raw: "https://example.com/?b=2&a=1&a=0", want: "a=0&a=1&b=2"},
		{name: "plus stays a literal plus", raw: "https://example.com/?q=hello+world", want: "q=hello%2Bworld"},
		{name: "encoded space stays a space", raw: "https://example.com/?q=hello%20world", want: "q=hello%20world"},
		{name: "base64 value survives", raw: "https://example.com/?t=YQ+b%2Fc%3D", want: "t=YQ%2Bb%2Fc%3D"},
		{name: "valueless param keeps the equals", raw: "https://example.com/?flag", want: "flag="},
		{name: "reserved characters are encoded", raw: "https://example.com/?p=a/b:c", want: "p=a%2Fb%3Ac"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.raw, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			if got := canonicalSigV4Query(req.URL); got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
