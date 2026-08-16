package sync

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/constants"
)

func TestUserAgent_IdentifiesAppAndPlatform(t *testing.T) {
	ua := userAgent()

	assert.True(t, strings.HasPrefix(ua, "Tetiva/"+constants.AppVersion+" ("+runtime.GOOS+"; "), "got %q", ua)
	assert.True(t, strings.HasSuffix(ua, ")"), "got %q", ua)
}
