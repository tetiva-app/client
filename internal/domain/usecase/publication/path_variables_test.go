package publication_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestBuild_PathVariables(t *testing.T) {
	f := newFixture()
	h := f.request(f.in.Root, "get user")
	h.URL = "https://api.example.com/users/:id/:tab"
	h.PathVariables = []entities.PathVariable{{Key: "id", Value: "{{userId}}"}, {Key: "tab", Value: ""}}
	ws := wsRequest(f, 0, 0)
	ws.URL = "wss://ws.example.com/rooms/:room"
	ws.PathVariables = []entities.PathVariable{{Key: "room", Value: "lobby"}}
	f.request(f.in.Root, "plain").PathVariables = []entities.PathVariable{}

	out, report := f.marshal(t)

	require.Empty(t, report.Errors)
	assert.Contains(t, out, `"url":"https://api.example.com/users/:id/:tab","pathVariables":[{"key":"id","value":"{{userId}}"},{"key":"tab","value":""}]`)
	assert.Contains(t, out, `"url":"wss://ws.example.com/rooms/:room","pathVariables":[{"key":"room","value":"lobby"}]`)
	assert.Equal(t, 2, strings.Count(out, `"pathVariables"`), "an empty list stays out, so older snapshots keep their hash")
}

func TestLimits_PathVariables(t *testing.T) {
	setup := func(n int, value string) func(f *fixture) {
		return func(f *fixture) {
			r := f.request(f.in.Root, "r")
			for i := range n {
				r.PathVariables = append(r.PathVariables, entities.PathVariable{Key: fmt.Sprintf("p%d", i), Value: value})
			}
		}
	}
	runLimitCases(t, map[string]limitCase{
		"value": {setup(1, strings.Repeat("я", 8193)), "r / path variables / p0"},
	}, "KiB")
	runLimitCases(t, map[string]limitCase{
		"count": {setup(201, "1"), "r / path variables"},
	}, "201 path variables")

	f := newFixture()
	setup(200, strings.Repeat("я", 8192))(f)
	_, report := f.build(t)
	assert.Empty(t, report.Errors)
}
