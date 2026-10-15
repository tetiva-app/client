package scriptengine

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestChaiIsTheVendoredRelease(t *testing.T) {
	sum := sha256.Sum256([]byte(chaiSource))
	assert.Equal(t, "bdc229d660afad0313fc10d6afb5a339956a18c4c6e819d3eb5d8b94f314c202", hex.EncodeToString(sum[:]))
}

func TestChaiLoadsOnlyWhenAScriptAsksForIt(t *testing.T) {
	cases := []struct {
		script string
		loaded bool
	}{
		{`pm.test("plain", function () { if (pm.response.code !== 200) throw new Error("no") })`, false},
		{`Object.keys(pm); Object.keys(pm.response); JSON.stringify(pm); JSON.stringify(pm.response)`, false},
		{`pm.expect(1).to.equal(1)`, true},
		{`pm.response.to.be.ok`, true},
	}
	for _, tc := range cases {
		vm := goja.New()
		sctx := request.ScriptContext{ResponseStatusCode: 200, ResponseBody: "{}"}
		loader := setupPmExpect(vm, setupPmResponse(vm, sctx), false)
		setupPmTest(vm, &runLog{})

		_, err := vm.RunString(tc.script)
		require.NoError(t, err, tc.script)
		assert.Equal(t, tc.loaded, loader.chai != nil, tc.script)
	}
}
