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

	got, warning := portability.ClampDescription(`request "Get"`, s)

	assert.Equal(t, s, got)
	assert.Empty(t, warning)
}

func TestClampDescription_CutsOnRuneBoundary(t *testing.T) {
	s := strings.Repeat("a", domain.MaxDescriptionLen-1) + "ж" + "tail"

	got, warning := portability.ClampDescription(`request "Get"`, s)

	assert.Equal(t, strings.Repeat("a", domain.MaxDescriptionLen-1), got)
	assert.True(t, utf8.ValidString(got))
	assert.Equal(t, `request "Get": description longer than 16384 bytes was truncated`, warning)
}
