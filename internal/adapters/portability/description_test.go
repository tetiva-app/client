package portability_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
)

func TestClampDescription_KeepsTextWithinLimit(t *testing.T) {
	s := strings.Repeat("a", domain.MaxDescriptionLen)

	got, warnings := portability.ClampDescription(portability.Subject{Kind: portability.KindRequest, Name: "Get"}, s)

	assert.Equal(t, s, got)
	assert.Empty(t, warnings)
}

func TestClampDescription_CutsOnRuneBoundary(t *testing.T) {
	s := strings.Repeat("a", domain.MaxDescriptionLen-1) + "ж" + "tail"

	got, warnings := portability.ClampDescription(portability.Subject{Kind: portability.KindRequest, Name: "Get"}, s)

	assert.Equal(t, strings.Repeat("a", domain.MaxDescriptionLen-1), got)
	assert.True(t, utf8.ValidString(got))
	assert.Equal(t, []string{`request "Get": description longer than 16,384 bytes was truncated`}, portability.Texts(warnings))
	assert.Equal(t, portability.WarnDescriptionTruncated, warnings[0].Code)
	assert.Equal(t, map[string]string{"max": "16384"}, warnings[0].Params)
}
