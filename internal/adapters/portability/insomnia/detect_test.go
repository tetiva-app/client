package insomnia_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetect_InsomniaExports(t *testing.T) {
	imp := newStore().importer()
	for _, name := range []string{"petstore.insomnia.json", "airlines.insomnia.yaml", "sleep.insomnia.yaml"} {
		assert.True(t, imp.Detect(fixture(t, name)), name)
	}
	cases := map[string]string{
		"bom and blank lines":  "\xef\xbb\xbf\n\ntype: collection.insomnia.rest/5.0\nname: x\n",
		"quoted type":          "type: \"collection.insomnia.rest/5.0\"\n",
		"design document":      "type: spec.insomnia.rest/5.0\nname: x\n",
		"v4 format first":      "__export_format: 4\n_type: export\nresources: []\n",
		"v4 json one line":     `{"_type":"export","__export_format":4,"resources":[]}`,
		"v4 json with spacing": "\n  {\n \"__export_format\": 4, \"_type\": \"export\", \"resources\": [] }",
	}
	for name, content := range cases {
		assert.True(t, imp.Detect([]byte(content)), name)
	}
}

func TestDetect_LeavesOtherFormatsAlone(t *testing.T) {
	imp := newStore().importer()

	postmanFiles, err := filepath.Glob(filepath.Join("..", "postman", "testdata", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, postmanFiles)
	snapshot := filepath.Join("..", "..", "..", "..", "testdata", "snapshot", "all-protocols.json")
	for _, path := range append(postmanFiles, snapshot) {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.False(t, imp.Detect(raw), path)
	}

	cases := map[string]string{
		"postman environment":   `{"name":"Dev","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"environment"}`,
		"har":                   `{"log":{"version":"1.2","entries":[]}}`,
		"openapi yaml":          "openapi: 3.0.4\ninfo:\n  title: x\n  version: 1.0.0\npaths: {}\n",
		"openapi json":          `{"openapi":"3.1.0","info":{"title":"x","version":"1"},"paths":{}}`,
		"insomnia environment":  "type: environment.insomnia.rest/5.0\nname: Globals\n",
		"insomnia mock server":  "type: mock.insomnia.rest/5.0\nname: Mock\n",
		"insomnia v3 export":    `{"_type":"export","__export_format":3,"resources":[]}`,
		"export without format": `{"_type":"export","resources":[]}`,
		"format as a string":    `{"_type":"export","__export_format":"4","resources":[]}`,
		"type not first":        "name: x\ntype: collection.insomnia.rest/5.0\n",
		"nested type":           "info:\n  type: collection.insomnia.rest/5.0\n",
		"yaml v3 export":        "_type: export\n__export_format: 3\nresources: []\n",
		"junk":                  "hello world",
		"empty":                 "",
		"broken json":           `{"_type":"export","__export_format":4,`,
	}
	for name, content := range cases {
		assert.False(t, imp.Detect([]byte(content)), name)
	}
}
