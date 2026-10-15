package scriptengine

import (
	_ "embed"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/dop251/goja"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// chai 4.5.0 from the npm tarball, unmodified; the sha256 is pinned in a test.
//
//go:embed chai/chai.js
var chaiSource string

//go:embed chai_postman.js
var chaiPostmanSource string

const (
	chaiSrcName        = "chai.js"
	chaiPostmanSrcName = "chai-postman.js"
)

var compileChai = sync.OnceValues(func() (*goja.Program, error) {
	return goja.Compile(chaiSrcName,
		"(function () { var module = { exports: {} }, exports = module.exports;\n"+chaiSource+"\nreturn module.exports; })()",
		false)
})

var compileChaiPostman = sync.OnceValues(func() (*goja.Program, error) {
	return goja.Compile(chaiPostmanSrcName, chaiPostmanSource, false)
})

type chaiLoader struct {
	vm       *goja.Runtime
	response goja.Value
	grpc     bool
	chai     *goja.Object
}

func setupPmExpect(vm *goja.Runtime, response *goja.Object, grpc bool) *chaiLoader {
	l := &chaiLoader{vm: vm, response: goja.Undefined(), grpc: grpc}
	_ = getPmObject(vm).DefineAccessorProperty("expect",
		vm.ToValue(func(goja.FunctionCall) goja.Value { return l.load().Get("expect") }),
		nil, goja.FLAG_TRUE, goja.FLAG_FALSE)

	if response != nil {
		l.response = response
		_ = response.DefineAccessorProperty("to",
			vm.ToValue(func(goja.FunctionCall) goja.Value { return l.responseTo() }),
			nil, goja.FLAG_TRUE, goja.FLAG_FALSE)
	}
	return l
}

func (l *chaiLoader) load() *goja.Object {
	if l.chai != nil {
		return l.chai
	}
	chai := l.run(compileChai)
	plugin, ok := goja.AssertFunction(l.run(compileChaiPostman))
	if !ok {
		panic(l.vm.NewTypeError(chaiPostmanSrcName + " is not a function"))
	}
	if _, err := plugin(goja.Undefined(), chai, l.response, l.vm.ToValue(l.grpc)); err != nil {
		panic(err)
	}
	l.chai = chai.ToObject(l.vm)
	return l.chai
}

// Errors are re-panicked as is: an interrupt must stay uncatchable for the script.
func (l *chaiLoader) run(compile func() (*goja.Program, error)) goja.Value {
	prg, err := compile()
	if err != nil {
		panic(l.vm.NewGoError(err))
	}
	v, err := l.vm.RunProgram(prg)
	if err != nil {
		panic(err)
	}
	return v
}

func (l *chaiLoader) responseTo() goja.Value {
	expect, ok := goja.AssertFunction(l.load().Get("expect"))
	if !ok {
		panic(l.vm.NewTypeError("chai.expect is not a function"))
	}
	assertion, err := expect(goja.Undefined(), l.response)
	if err != nil {
		panic(err)
	}
	return assertion.ToObject(l.vm).Get("to")
}

func reasonPhrase(sctx request.ScriptContext) string {
	reason := strings.TrimSpace(strings.TrimPrefix(sctx.ResponseStatusText, strconv.Itoa(sctx.ResponseStatusCode)))
	if reason == "" {
		return http.StatusText(sctx.ResponseStatusCode)
	}
	return reason
}
