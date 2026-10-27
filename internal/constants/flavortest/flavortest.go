package flavortest

import (
	"testing"

	"github.com/tetiva-app/client/internal/constants"
)

// Preview is not for parallel tests: the flavor is a process-wide variable.
func Preview(t testing.TB) {
	t.Helper()
	prev := constants.Flavor
	constants.Flavor = constants.FlavorPreview
	t.Cleanup(func() { constants.Flavor = prev })
}
