package portability_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/race"
)

type fakeImporter struct {
	name    string
	detects bool
}

func (f *fakeImporter) Detect([]byte) bool { return f.detects }

func (f *fakeImporter) Preview([]byte, string) (*portability.ImportPreview, error) {
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
		"{{baseUrl}}/pets", "{{baseUrl}}/users", "https://auth.example.com/token", "",
	}, vars)

	assert.Equal(t, []string{"auth.example.com", "petstore.example.com"}, got)
}

func TestHosts_LeavesOutHostsItCannotResolve(t *testing.T) {
	got := portability.Hosts([]string{
		"{{unknown}}/x", "https://{{host}}/y", "dns:///{{grpcHost}}:443", "{{scheme}}://{{host}}:{{port}}/w", "https://api.example.com/z",
	}, map[string]string{"host": "{{nested}}"})

	assert.Equal(t, []string{"api.example.com"}, got)
	assert.Equal(t, []string{}, portability.Hosts([]string{"{{baseUrl}}/pets"}, nil))
}

func TestHosts_KeepsHostsWithLiteralTextAroundAVariable(t *testing.T) {
	got := portability.Hosts([]string{
		"https://{{host}}/x", "https://api.{{region}}.example.com/y", "{{scheme}}://{{sub}}.example.org:8443/z",
	}, map[string]string{"host": "{{tenant}}.evil.com"})

	assert.Equal(t, []string{"api.{{region}}.example.com", "{{sub}}.example.org:8443", "{{tenant}}.evil.com"}, got)
}

func TestHosts_CapsWhatVariablesExpandInto(t *testing.T) {
	vars := map[string]string{"a": strings.Repeat("x", 64<<10)}
	urls := append(slices.Repeat([]string{strings.Repeat("{{a}}", 100) + "/x"}, 200), "https://api.example.com/y")

	start := time.Now()
	got := portability.Hosts(urls, vars)

	assert.Less(t, time.Since(start), time.Second*race.Slowdown)
	assert.Equal(t, []string{"api.example.com"}, got)
}

func TestFreeName(t *testing.T) {
	cases := []struct {
		name  string
		taken []string
		want  string
	}{
		{"Dev", nil, "Dev"},
		{"Dev", []string{"Prod"}, "Dev"},
		{"Dev", []string{"Dev"}, "Dev (2)"},
		{"Dev", []string{"Dev", "Dev (2)"}, "Dev (3)"},
		{"Dev", []string{"Dev", "Dev (3)"}, "Dev (2)"},
		{"Dev", []string{"dev"}, "Dev"},
		{"Dev (2)", []string{"Dev (2)"}, "Dev (2) (2)"},
	}
	for _, tc := range cases {
		taken := map[string]bool{}
		for _, n := range tc.taken {
			taken[n] = true
		}
		assert.Equal(t, tc.want, portability.FreeName(tc.name, taken), "%s taken=%v", tc.name, tc.taken)
	}
}

func TestListing(t *testing.T) {
	assert.Equal(t, "", portability.Listing(nil))
	assert.Equal(t, "a, b", portability.Listing([]string{"a", "b"}))
	assert.Equal(t, "a, b, c, d, e and 2 more", portability.Listing([]string{"a", "b", "c", "d", "e", "f", "g"}))
}

func TestCheckFileSize_LeavesRoomForTheEscapedCallBody(t *testing.T) {
	require.NoError(t, portability.CheckFileSize(48<<20))

	var reasonErr *domain.ReasonError
	require.ErrorAs(t, portability.CheckFileSize(48<<20+1), &reasonErr)
	assert.Equal(t, portability.ReasonFileTooLarge, reasonErr.Reason)
	var validation *domain.ValidationError
	require.ErrorAs(t, reasonErr, &validation)
	assert.Equal(t, "larger than 48 MiB", validation.Fields["content"])
}
