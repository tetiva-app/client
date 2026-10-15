package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
)

func TestImportWarningsFrom(t *testing.T) {
	got := ImportWarningsFrom([]portability.Warning{
		portability.Warn(portability.WarnFileFieldDropped, portability.Subject{Kind: portability.KindRequest, Name: "Upload"}, "field", "photo"),
		portability.Warn(portability.WarnHoppscotchHeadersCopied, portability.Subject{}),
	})

	assert.Equal(t, []ImportWarning{
		{
			Code: "file_field_dropped", Subject: ImportWarningSubject{Kind: "request", Name: "Upload"},
			Params: map[string]string{"field": "photo"},
			Text:   `request "Upload": file field "photo" was imported without its file; pick it again`,
		},
		{
			Code: "hoppscotch_headers_copied", Subject: ImportWarningSubject{}, Params: map[string]string{},
			Text: "headers set on Hoppscotch collections and folders were copied into their requests",
		},
	}, got)
}

func TestImportWarningsFrom_NeverSendsNull(t *testing.T) {
	raw, err := json.Marshal(ImportWarningsFrom(nil))
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(raw))

	raw, err = json.Marshal(ImportWarningsFrom([]portability.Warning{{Code: portability.WarnOpenAPINoServer, Text: "t"}}))
	require.NoError(t, err)
	assert.JSONEq(t, `[{"code":"openapi_no_server","subject":{"kind":"","name":""},"params":{},"text":"t"}]`, string(raw))
}
