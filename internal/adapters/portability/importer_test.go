package portability_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
)

type fakeImporter struct {
	name    string
	detects bool
}

func (f *fakeImporter) Detect([]byte) bool { return f.detects }

func (f *fakeImporter) Preview([]byte) (*portability.ImportPreview, error) {
	return &portability.ImportPreview{Format: f.name}, nil
}

func (f *fakeImporter) Import(context.Context, []byte, portability.ImportOpt) (*portability.ImportResult, error) {
	return &portability.ImportResult{}, nil
}

func TestSelect_FirstDetectWins(t *testing.T) {
	a := &fakeImporter{name: "a"}
	b := &fakeImporter{name: "b", detects: true}
	c := &fakeImporter{name: "c", detects: true}

	got, err := portability.Select([]byte("{}"), a, b, c)
	require.NoError(t, err)
	assert.Same(t, b, got)
}

func TestSelect_NoneDetects(t *testing.T) {
	_, err := portability.Select([]byte("{}"), &fakeImporter{}, &fakeImporter{})

	var re *domain.ReasonError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, portability.ReasonUnsupportedFile, re.Reason)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
}

func TestHostOf(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com/v1/pets?x=1":  "api.example.com",
		"http://user:pw@example.com:8080/path": "example.com:8080",
		"wss://ws.example.com/events#frag":     "ws.example.com",
		"grpc.example.com:443":                 "grpc.example.com:443",
		"{{baseUrl}}/pets":                     "{{baseUrl}}",
		"https://{{host}}/x":                   "{{host}}",
		"  HTTPS://API.Example.COM/  ":         "api.example.com",
		"":                                     "",
	}
	for in, want := range cases {
		assert.Equal(t, want, portability.HostOf(in), in)
	}
}

func TestHostOf_GRPCResolverTargetsShowTheDialledEndpoint(t *testing.T) {
	cases := map[string]string{
		"dns:///evil.example.com:443":             "evil.example.com:443",
		"dns://1.1.1.1/evil.example.com:443":      "evil.example.com:443",
		"passthrough:///evil.example.com:443":     "evil.example.com:443",
		"xds:///svc.example.com":                  "svc.example.com",
		"DNS:///Evil.Example.COM:443":             "evil.example.com:443",
		"dns:/evil.example.com:443":               "evil.example.com:443",
		"dns:evil.example.com:443":                "evil.example.com:443",
		"dns:///evil%2Eexample.com:443":           "evil.example.com:443",
		"dns:///evil.example.com:443?x=1#f":       "evil.example.com:443",
		"dns://{{resolver}}/evil.example.com:443": "evil.example.com:443",
		"dns:///{{host}}:443":                     "{{host}}:443",
		"dns.example.com:443":                     "dns.example.com:443",
	}
	for in, want := range cases {
		assert.Equal(t, want, portability.HostOf(in), in)
	}
}

func TestHostOf_WhatItCannotReduceComesBackWhole(t *testing.T) {
	for _, in := range []string{
		"/relative/only", "https://", "https://user@/x", "dns:///", "dns://1.1.1.1", "dns://1.1.1.1?/e.com:443",
		"unix:///run/app.sock", "unix:/run/app.sock", "unix-abstract:app",
	} {
		assert.Equal(t, in, portability.HostOf(in))
	}
}

func TestHosts_SubstitutesPublicVariablesSortsAndDedupes(t *testing.T) {
	vars := map[string]string{"baseUrl": "https://petstore.example.com/v1"}

	got := portability.Hosts([]string{
		"{{baseUrl}}/pets", "{{baseUrl}}/users", "https://auth.example.com/token", "{{unknown}}/x", "",
	}, vars)

	assert.Equal(t, []string{"auth.example.com", "petstore.example.com", "{{unknown}}"}, got)
}
