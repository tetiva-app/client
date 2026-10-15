package insomnia_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/race"
)

func TestPreview_CountsMatchTheImport(t *testing.T) {
	for _, name := range []string{"petstore.insomnia.json", "airlines.insomnia.yaml", "sleep.insomnia.yaml"} {
		t.Run(name, func(t *testing.T) {
			s := newStore()
			data := fixture(t, name)

			preview, err := s.importer().Preview(data, "")
			require.NoError(t, err)
			res := importFile(t, s, data, true)

			assert.Equal(t, portability.FormatInsomnia, preview.Format)
			assert.Equal(t, res.Folders, preview.Folders)
			assert.Equal(t, res.Requests, preview.Requests)
			assert.Zero(t, preview.Examples)
			assert.Equal(t, res.EnvironmentNames, preview.EnvironmentNames)
			assert.Equal(t, res.Warnings, preview.Warnings)
			assert.Len(t, s.collections, res.Folders+1)
		})
	}
}

func TestPreview_Petstore(t *testing.T) {
	preview, err := newStore().importer().Preview(fixture(t, "petstore.insomnia.json"), "")
	require.NoError(t, err)

	assert.Equal(t, "Petstore", preview.Title)
	assert.Equal(t, 2, preview.Folders)
	assert.Equal(t, 7, preview.Requests)
	assert.Equal(t, []string{"Petstore · Local", "Petstore · Production"}, preview.EnvironmentNames)
	assert.Equal(t, []string{
		"auth.example.com", "grpc.example.com:443", "localhost:8080", "localhost:9000",
		"petstore.example.com", "ws.example.com",
	}, preview.Hosts)
	assert.Equal(t, []portability.ScriptPreview{
		{Path: "Petstore / Pets", Phase: "pre", Text: "insomnia.request.addHeader({ key: 'X-Folder', value: 'pets' });"},
		{Path: "Petstore / Pets / Add pet", Phase: "pre", Text: "insomnia.environment.set('started', Date.now());"},
		{Path: "Petstore / Pets / Add pet", Phase: "post", Text: "insomnia.test('created', () => {\n  insomnia.expect(insomnia.response.code).to.eql(201);\n});"},
	}, preview.Scripts)
	assert.Contains(t, portability.Texts(preview.Warnings), scriptsWarning)
}

func TestPreview_EmptyListsAreNotNil(t *testing.T) {
	preview, err := newStore().importer().Preview([]byte(`{"_type":"export","__export_format":4,"resources":[`+
		`{"_id":"wrk_1","_type":"workspace","name":"Empty"}]}`), "")
	require.NoError(t, err)

	assert.Equal(t, "Empty", preview.Title)
	assert.Zero(t, preview.Folders)
	assert.Empty(t, preview.EnvironmentNames)
	assert.NotNil(t, preview.Hosts)
	assert.NotNil(t, preview.Scripts)
	assert.NotNil(t, preview.Warnings)
}

func TestPreview_UnreadableFiles(t *testing.T) {
	cases := map[string]string{
		"broken yaml":        "type: collection.insomnia.rest/5.0\ncollection: [\n",
		"no workspace":       `{"_type":"export","__export_format":4,"resources":[{"_id":"env_1","_type":"environment","data":{}}]}`,
		"resources not list": `{"_type":"export","__export_format":4,"resources":{"a":1}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newStore().importer().Preview([]byte(content), "")

			var ve *domain.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Contains(t, ve.Fields["content"], "Insomnia export")
		})
	}
}

func TestPreview_EnvironmentDataLimits(t *testing.T) {
	const head = "type: collection.insomnia.rest/5.0\nname: x\n"
	var bomb strings.Builder
	bomb.WriteString(head + "x0: &a0 {k0: lol, k1: lol, k2: lol, k3: lol, k4: lol, k5: lol, k6: lol, k7: lol, k8: lol}\n")
	for i := 1; i < 7; i++ {
		fmt.Fprintf(&bomb, "x%d: &a%d {", i, i)
		for j := range 9 {
			fmt.Fprintf(&bomb, "k%d: *a%d, ", j, i-1)
		}
		bomb.WriteString("}\n")
	}
	bomb.WriteString("environments:\n  name: base\n  data: *a6\n")
	var many strings.Builder
	many.WriteString(head + "environments:\n  name: base\n  data:\n")
	for i := range 10001 {
		fmt.Fprintf(&many, "    k%d: v\n", i)
	}
	deep := head + "environments:\n  name: base\n  data: " + strings.Repeat("{a: ", 40) + "x" + strings.Repeat("}", 40) + "\n"

	cases := map[string]struct{ doc, want string }{
		"self-referencing anchor":  {head + "environments:\n  name: base\n  data: &a\n    k: *a\n", "anchor 'a' value contains itself"},
		"repeated alias expansion": {bomb.String(), "excessive aliasing"},
		"too many variables":       {many.String(), "more than 10000 variables"},
		"too deep":                 {deep, "nests deeper than 32 levels"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newStore().importer().Preview([]byte(tc.doc), "")

			var ve *domain.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Contains(t, ve.Fields["content"], tc.want)
		})
	}
}

func TestPreview_EnvironmentNamesWithCommas(t *testing.T) {
	data := []byte(`{"_type":"export","__export_format":4,"resources":[` +
		`{"_id":"wrk_1","_type":"workspace","name":"Users, Orders"},` +
		`{"_id":"env_1","_type":"environment","parentId":"wrk_1","name":"Base","data":{"a":"1"}},` +
		`{"_id":"env_2","_type":"environment","parentId":"env_1","name":"Local, EU","data":{"b":"2"}},` +
		`{"_id":"env_3","_type":"environment","parentId":"env_1","name":"Prod","data":{"b":"3"}}]}`)

	preview, err := newStore().importer().Preview(data, "")
	require.NoError(t, err)

	assert.Equal(t, []string{"Users, Orders · Local, EU", "Users, Orders · Prod"}, preview.EnvironmentNames)
}

func TestPreview_VariablesExpandingIntoThemselvesStayBounded(t *testing.T) {
	var subs strings.Builder
	for i := range 30 {
		fmt.Fprintf(&subs, `,{"_id":"env_s%d","_type":"environment","parentId":"env_1","name":"S%d","data":{}}`, i, i)
	}
	data := []byte(`{"_type":"export","__export_format":4,"resources":[` +
		`{"_id":"wrk_1","_type":"workspace","name":"W"},` +
		`{"_id":"env_1","_type":"environment","parentId":"wrk_1","name":"Base","data":{` +
		`"a":"` + strings.Repeat("{{a}}", 20) + `","host":"api.example.com","base":"https://{{host}}"}}` + subs.String() + `,` +
		`{"_id":"req_1","_type":"request","parentId":"wrk_1","name":"A","method":"GET","url":"{{a}}/x"},` +
		`{"_id":"req_2","_type":"request","parentId":"wrk_1","name":"B","method":"GET","url":"{{base}}/y"}]}`)

	start := time.Now()
	preview, err := newStore().importer().Preview(data, "")
	require.NoError(t, err)

	assert.Less(t, time.Since(start), time.Second*race.Slowdown)
	assert.Equal(t, []string{"api.example.com"}, preview.Hosts)
}
