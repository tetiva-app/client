package wails

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/har"
	"github.com/tetiva-app/client/internal/adapters/portability/hoppscotch"
	"github.com/tetiva-app/client/internal/adapters/portability/insomnia"
	"github.com/tetiva-app/client/internal/adapters/portability/openapi"
	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/adapters/portability/snapshot"
)

func importerFormat(t *testing.T, imp portability.Importer) string {
	t.Helper()
	switch imp.(type) {
	case *snapshot.Importer:
		return portability.FormatTetiva
	case *postman.Importer:
		return portability.FormatPostman
	case *insomnia.Importer:
		return portability.FormatInsomnia
	case *openapi.Importer:
		return portability.FormatOpenAPI
	case *har.Importer:
		return portability.FormatHAR
	case *hoppscotch.Importer:
		return portability.FormatHoppscotch
	}
	t.Fatalf("unknown importer %T", imp)
	return ""
}

func TestImporters_DetectOrder(t *testing.T) {
	s := NewPortabilityService(nil, nil, nil, nil, nil, nil)

	var order []string
	for _, imp := range s.importers {
		order = append(order, importerFormat(t, imp))
	}

	assert.Equal(t, []string{
		portability.FormatTetiva, portability.FormatPostman, portability.FormatInsomnia,
		portability.FormatOpenAPI, portability.FormatHAR, portability.FormatHoppscotch,
	}, order)
}

func TestImporters_EverySampleHasExactlyOneOwner(t *testing.T) {
	s := NewPortabilityService(nil, nil, nil, nil, nil, nil)
	samples := map[string]string{
		"../../../testdata/snapshot/*.json":    portability.FormatTetiva,
		"../portability/postman/testdata/*":    portability.FormatPostman,
		"../portability/insomnia/testdata/*":   portability.FormatInsomnia,
		"../portability/openapi/testdata/*":    portability.FormatOpenAPI,
		"../portability/har/testdata/*":        portability.FormatHAR,
		"../portability/hoppscotch/testdata/*": portability.FormatHoppscotch,
	}
	notCollections := map[string]bool{
		"collection-snapshot.v1.schema.json": true,
		"environments.json":                  true,
	}
	require.Len(t, samples, len(s.importers))

	for pattern, format := range samples {
		paths, err := filepath.Glob(filepath.FromSlash(pattern))
		require.NoError(t, err)
		require.NotEmpty(t, paths, pattern)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			require.NoError(t, err)

			var owners []string
			for _, imp := range s.importers {
				if imp.Detect(data) {
					owners = append(owners, importerFormat(t, imp))
				}
			}

			if notCollections[filepath.Base(path)] {
				assert.Empty(t, owners, path)
				continue
			}
			assert.Equal(t, []string{format}, owners, path)
			selected, err := portability.Select(data, s.importers...)
			require.NoError(t, err, path)
			assert.Equal(t, format, importerFormat(t, selected), path)
		}
	}
}

func TestImporters_EnvironmentFilesHaveNoOwner(t *testing.T) {
	s := NewPortabilityService(nil, nil, nil, nil, nil, nil)
	files := map[string]string{
		"postman environment": `{"name":"Dev","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"environment"}`,
		"postman globals":     `{"name":"","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"globals"}`,
		"hoppscotch single":   `{"v":2,"id":"e1","name":"Dev","variables":[{"key":"a","initialValue":"1","secret":false}]}`,
		"insomnia v5":         "type: environment.insomnia.rest/5.0\nname: Globals\n",
	}

	for name, content := range files {
		_, err := portability.Select([]byte(content), s.importers...)
		assert.Error(t, err, name)
	}
}
