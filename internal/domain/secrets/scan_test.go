package secrets

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

func TestScan_EachRule(t *testing.T) {
	cases := []struct {
		rule, text, secret string
	}{
		{"jwt", `{"token_value":"` + testJWT + `"}`, testJWT},
		{"aws-access-key", "key=AKIAIOSFODNN7EXAMPLE;", "AKIAIOSFODNN7EXAMPLE"},
		{"aws-access-key", "ASIAIOSFODNN7EXAMPLE", "ASIAIOSFODNN7EXAMPLE"},
		{"github-token", "token ghp_" + strings.Repeat("a1B2", 9), "ghp_" + strings.Repeat("a1B2", 9)},
		{"github-token", "gho_" + strings.Repeat("Z", 36), "gho_" + strings.Repeat("Z", 36)},
		{"slack-token", "xoxb-1234567890-abcdefghij", "xoxb-1234567890-abcdefghij"},
		{"stripe-key", "sk_live_" + strings.Repeat("4eC39HqLyjWD", 2), "sk_live_" + strings.Repeat("4eC39HqLyjWD", 2)},
		{"google-api-key", "AIza" + strings.Repeat("S", 35), "AIza" + strings.Repeat("S", 35)},
		{"telegram-bot-token", "/bot123456789:" + strings.Repeat("A", 35) + "/getMe", "123456789:" + strings.Repeat("A", 35)},
		{"bearer-token", "Authorization: Bearer abcdefghij0123456789xyz", "abcdefghij0123456789xyz"},
		{"bearer-token", "bearer\tabcdefghij0123456789==", "abcdefghij0123456789=="},
		{"oauth-token-field", `{"access_token": "2YotnFZFEjr1zCsicMWpAA"}`, "2YotnFZFEjr1zCsicMWpAA"},
		{"oauth-token-field", `{"refresh_token":"tGzv3JOkF0XG5Qx2TlKWIA"}`, "tGzv3JOkF0XG5Qx2TlKWIA"},
		{"oauth-token-field", `{"id_token" : "opaque"}`, "opaque"},
		{"oauth-token-field", `{"accessToken":"a\"b"}`, `a\"b`},
		{"private-key", "-----BEGIN PRIVATE KEY-----\\nMIIEv\\n-----END PRIVATE KEY-----",
			"-----BEGIN PRIVATE KEY-----\\nMIIEv\\n-----END PRIVATE KEY-----"},
		{"private-key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEo", "-----BEGIN RSA PRIVATE KEY-----"},
		{"private-key", "-----BEGIN OPENSSH PRIVATE KEY-----", "-----BEGIN OPENSSH PRIVATE KEY-----"},
	}
	for _, c := range cases {
		findings := Scan(c.text)
		var hit *Finding
		for i := range findings {
			if findings[i].Rule == c.rule {
				hit = &findings[i]
				break
			}
		}
		require.NotNil(t, hit, "rule %s in %q: got %+v", c.rule, c.text, findings)
		assert.Equal(t, c.secret, c.text[hit.Start:hit.End], "rule %s", c.rule)
		assert.NotEmpty(t, hit.Label)
	}
}

func TestScan_PlainJSONHasNoFindings(t *testing.T) {
	texts := []string{
		`{"id":42,"name":"Alice","email":"alice@example.com","tags":["admin","beta"],"active":true}`,
		`{"created_at":"2026-09-25T10:00:00Z","uuid":"3fa85f64-5717-4562-b3fc-2c963f66afa6","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`,
		`{"token_type":"Bearer","expires_in":3600,"scope":"read write","access_token":"","refresh_token":"{{refresh}}"}`,
		`{"access_token":"<redacted>","id_token":"[redacted]"}`,
		`{"message":"Send a Bearer token in the Authorization header","docs":"https://example.com/auth#bearer"}`,
		`{"version":"1.2.3","build":"20260925.1","hash":"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"}`,
		`{"image":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGAKIAAAAASUVORK5CYII="}`,
		`{"phone":"+1 555 0100","zip":"94107","ids":[1234567890,2345678901]}`,
		"Bearer {{token}}", "Bearer <redacted>", "",
	}
	for _, text := range texts {
		assert.Empty(t, Scan(text), "text %q", text)
	}
}

func TestScan_OrdersByPositionAndLabelsDeduplicate(t *testing.T) {
	text := `{"a":"AKIAIOSFODNN7EXAMPLE","b":"` + testJWT + `","c":"AKIAIOSFODNN7EXAMPLF"}`

	findings := Scan(text)
	require.Len(t, findings, 3)
	for i := 1; i < len(findings); i++ {
		assert.LessOrEqual(t, findings[i-1].Start, findings[i].Start)
	}
	assert.Equal(t, []string{"AWS access key", "JWT"}, Labels(findings))
	assert.Empty(t, Labels(nil))
}

func TestScan_PrivateKeyPairsEachHeaderWithTheNextEnd(t *testing.T) {
	const (
		begin    = "-----BEGIN RSA PRIVATE KEY-----"
		end      = "-----END RSA PRIVATE KEY-----"
		innerHdr = "-----BEGIN PRIVATE KEY-----"
	)
	text := end + " x " + begin + "\nAAA\n" + innerHdr + "\nBBB\n" + end + " y " + begin + "\nCCC"

	var keys []string
	for _, f := range Scan(text) {
		if f.Rule == "private-key" {
			keys = append(keys, text[f.Start:f.End])
		}
	}
	assert.Equal(t, []string{begin + "\nAAA\n" + innerHdr + "\nBBB\n" + end, begin}, keys)
}

func TestScan_UnclosedPrivateKeyHeadersStayLinear(t *testing.T) {
	const header = "-----BEGIN PRIVATE KEY-----"
	text := strings.Repeat(header, 256*1024/len(header))
	baseline := strings.Repeat("-----BEGIN PUBLIC KEY-----.", 256*1024/len(header))

	start := time.Now()
	assert.Empty(t, Scan(baseline))
	plain := time.Since(start)

	start = time.Now()
	findings := Scan(text)
	elapsed := time.Since(start)

	assert.Len(t, findings, 256*1024/len(header))
	// Relative to a same-size text, so the bound holds under -race and on a busy machine.
	assert.Less(t, elapsed, 10*plain)
}
