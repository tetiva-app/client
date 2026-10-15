package scriptengine

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const (
	scriptTimeout = 5 * time.Second
	// Interrupt is checked between bytecode instructions, so it never lands inside Go code —
	// a regexp with backtracking is the known case. After this grace period we stop waiting.
	interruptGrace   = 250 * time.Millisecond
	maxConsoleLines  = 1000
	maxConsoleValues = 10000
)

var abandonedScripts atomic.Int64

// How many script runs ignored the interrupt and were left running.
func AbandonedScripts() int64 { return abandonedScripts.Load() }

type GojaEngine struct{}

func NewGojaEngine() *GojaEngine {
	return &GojaEngine{}
}

func (e *GojaEngine) RunPreScript(ctx context.Context, script string, sctx request.ScriptContext) (*request.PreScriptResult, error) {
	vm := goja.New()

	vars := copyMap(sctx.Variables)
	locals := map[string]string{}
	headers := copyHeaders(sctx.RequestHeaders)
	sctx.Metadata = copyHeaders(sctx.Metadata)

	var log runLog

	setupConsole(vm, &log)
	setupBase64(vm)
	setupPmEnvironment(vm, vars, locals)
	setupPmRequestPreScript(vm, sctx, headers)
	setupPmExpect(vm, nil, false)

	if err := runWithTimeout(ctx, vm, script, scriptTimeout); err != nil {
		console, _ := log.snapshot()
		return &request.PreScriptResult{ConsoleOutput: console}, err
	}

	console, _ := log.snapshot()
	return &request.PreScriptResult{
		Headers:        headers,
		Metadata:       sctx.Metadata,
		Variables:      vars,
		LocalVariables: locals,
		ConsoleOutput:  console,
	}, nil
}

func (e *GojaEngine) RunPostScript(ctx context.Context, script string, sctx request.ScriptContext) (*request.PostScriptResult, error) {
	vm := goja.New()

	vars := copyMap(sctx.Variables)
	headers := copyHeaders(sctx.RequestHeaders)
	sctx.Metadata = copyHeaders(sctx.Metadata)

	var log runLog

	setupConsole(vm, &log)
	setupBase64(vm)
	setupPmEnvironment(vm, vars, copyMap(sctx.LocalVariables))
	setupPmRequestPreScript(vm, sctx, headers)
	setupPmExpect(vm, setupPmResponse(vm, sctx), sctx.Protocol == "grpc")
	setupPmTest(vm, &log)

	if err := runWithTimeout(ctx, vm, script, scriptTimeout); err != nil {
		console, tests := log.snapshot()
		return &request.PostScriptResult{TestResults: tests, ConsoleOutput: console}, err
	}

	console, tests := log.snapshot()
	return &request.PostScriptResult{
		TestResults:   tests,
		Variables:     vars,
		ConsoleOutput: console,
	}, nil
}

// runLog is locked because an abandoned run keeps writing to it.
type runLog struct {
	mu      sync.Mutex
	console []string
	dropped int
	tests   []request.TestResult
}

func (l *runLog) log(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.console) >= maxConsoleLines {
		l.dropped++
		return
	}
	l.console = append(l.console, line)
}

func (l *runLog) test(r request.TestResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tests = append(l.tests, r)
}

func (l *runLog) snapshot() ([]string, []request.TestResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	console := slices.Clone(l.console)
	if l.dropped > 0 {
		console = append(console, fmt.Sprintf("… %d more lines not shown", l.dropped))
	}
	return console, slices.Clone(l.tests)
}

func setupConsole(vm *goja.Runtime, log *runLog) {
	stringify, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("stringify"))
	logFn := func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = consoleText(vm, stringify, arg)
		}
		log.log(strings.Join(parts, " "))
		return goja.Undefined()
	}

	console := vm.NewObject()
	_ = console.Set("log", logFn)
	_ = console.Set("warn", logFn)
	_ = console.Set("error", logFn)
	_ = console.Set("info", logFn)
	_ = vm.Set("console", console)
}

// btoa and atob follow the browser contract: one byte per Latin1 character.
func setupBase64(vm *goja.Runtime) {
	_ = vm.Set("btoa", func(call goja.FunctionCall) goja.Value {
		s := call.Argument(0).String()
		b := make([]byte, 0, len(s))
		for _, r := range s {
			if r > 0xFF {
				panic(vm.NewTypeError("btoa: the string contains characters outside of the Latin1 range"))
			}
			b = append(b, byte(r))
		}
		return vm.ToValue(base64.StdEncoding.EncodeToString(b))
	})
	_ = vm.Set("atob", func(call goja.FunctionCall) goja.Value {
		b, ok := forgivingBase64(call.Argument(0).String())
		if !ok {
			panic(vm.NewTypeError("atob: the string is not valid base64"))
		}
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		return vm.ToValue(string(r))
	})
}

// The serializer runs in Go, out of the timeout's reach, so the replacer caps the work.
func consoleText(vm *goja.Runtime, stringify goja.Callable, v goja.Value) string {
	obj, ok := v.(*goja.Object)
	if !ok || obj.ClassName() == "Function" || obj.ClassName() == "Error" {
		return v.String()
	}
	visited := 0
	limit := vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if visited++; visited > maxConsoleValues {
			panic(vm.NewTypeError("too large to print"))
		}
		return call.Argument(1)
	})
	out, err := stringify(goja.Undefined(), v, limit)
	if err != nil || goja.IsUndefined(out) {
		return v.String()
	}
	return out.String()
}

// Postman's Buffer-based atob also takes base64url, unlike a browser's.
func forgivingBase64(s string) ([]byte, bool) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\f', '\r':
			return -1
		case '-':
			return '+'
		case '_':
			return '/'
		}
		return r
	}, s)
	s = strings.TrimRight(s, "=")
	if len(s)%4 == 1 {
		return nil, false
	}
	b, err := base64.RawStdEncoding.DecodeString(s)
	return b, err == nil
}

func setupPmEnvironment(vm *goja.Runtime, vars, locals map[string]string) {
	env := vm.NewObject()
	_ = env.Set("get", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		val, ok := vars[key]
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(val)
	})
	_ = env.Set("set", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		value := call.Argument(1).String()
		vars[key] = value
		return goja.Undefined()
	})
	_ = env.Set("unset", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		delete(vars, key)
		return goja.Undefined()
	})

	local := vm.NewObject()
	_ = local.Set("get", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		if val, ok := locals[key]; ok {
			return vm.ToValue(val)
		}
		if val, ok := vars[key]; ok {
			return vm.ToValue(val)
		}
		return goja.Undefined()
	})
	_ = local.Set("set", func(call goja.FunctionCall) goja.Value {
		locals[call.Argument(0).String()] = call.Argument(1).String()
		return goja.Undefined()
	})
	_ = local.Set("unset", func(call goja.FunctionCall) goja.Value {
		delete(locals, call.Argument(0).String())
		return goja.Undefined()
	})
	merged := func() map[string]string {
		out := copyMap(vars)
		for k, v := range locals {
			out[k] = v
		}
		return out
	}
	_ = local.Set("replaceIn", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(request.SubstituteVariables(call.Argument(0).String(), merged()))
	})
	_ = local.Set("toObject", func(goja.FunctionCall) goja.Value {
		obj := vm.NewObject()
		for k, v := range merged() {
			_ = obj.Set(k, v)
		}
		return obj
	})
	_ = local.Set("has", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		_, isLocal := locals[key]
		_, isVar := vars[key]
		return vm.ToValue(isLocal || isVar)
	})

	pm := getPmObject(vm)
	_ = pm.Set("environment", env)
	_ = pm.Set("collectionVariables", env)
	_ = pm.Set("variables", local)
	setupUnsupportedPm(vm, pm)
}

func setupUnsupportedPm(vm *goja.Runtime, pm *goja.Object) {
	unsupported := func(msg string) func(goja.FunctionCall) goja.Value {
		return func(goja.FunctionCall) goja.Value { panic(vm.NewTypeError(msg)) }
	}
	globals := vm.NewObject()
	for _, name := range []string{"get", "set", "unset", "has", "clear", "toObject"} {
		_ = globals.Set(name, unsupported("pm.globals is not supported in Tetiva; use pm.environment"))
	}
	_ = pm.Set("globals", globals)
	_ = pm.Set("sendRequest", unsupported("pm.sendRequest is not supported in Tetiva: scripts cannot make network calls"))
}

func setupPmRequestPreScript(vm *goja.Runtime, sctx request.ScriptContext, headers map[string][]string) {
	reqHeaders := vm.NewObject()
	_ = reqHeaders.Set("upsert", func(call goja.FunctionCall) goja.Value {
		key, value := headerArg(vm, call)
		for k := range headers {
			if strings.EqualFold(k, key) {
				key = k
				break
			}
		}
		deleteHeader(headers, key)
		headers[key] = []string{value}
		return goja.Undefined()
	})
	_ = reqHeaders.Set("add", func(call goja.FunctionCall) goja.Value {
		key, value := headerArg(vm, call)
		for k, v := range headers {
			if strings.EqualFold(k, key) {
				headers[k] = append(v, value)
				return goja.Undefined()
			}
		}
		headers[key] = []string{value}
		return goja.Undefined()
	})
	_ = reqHeaders.Set("remove", func(call goja.FunctionCall) goja.Value {
		deleteHeader(headers, call.Argument(0).String())
		return goja.Undefined()
	})

	reqObj := vm.NewObject()
	_ = reqObj.Set("method", sctx.RequestMethod)
	_ = reqObj.Set("url", sctx.RequestURL)
	_ = reqObj.Set("headers", reqHeaders)
	_ = reqObj.Set("protocol", sctx.Protocol)

	if sctx.Protocol == "grpc" {
		_ = reqObj.Set("message", sctx.Message)
		_ = reqObj.Set("service", sctx.Service)
		_ = reqObj.Set("grpcMethod", sctx.GRPCMethod)

		metadataObj := vm.NewObject()
		_ = metadataObj.Set("get", func(call goja.FunctionCall) goja.Value {
			key := call.Argument(0).String()
			if vals, ok := sctx.Metadata[key]; ok && len(vals) > 0 {
				return vm.ToValue(vals[0])
			}
			return goja.Undefined()
		})
		_ = metadataObj.Set("set", func(call goja.FunctionCall) goja.Value {
			key := call.Argument(0).String()
			val := call.Argument(1).String()
			sctx.Metadata[key] = []string{val}
			return goja.Undefined()
		})
		_ = metadataObj.Set("remove", func(call goja.FunctionCall) goja.Value {
			key := call.Argument(0).String()
			delete(sctx.Metadata, key)
			return goja.Undefined()
		})
		_ = metadataObj.Set("toObject", func(call goja.FunctionCall) goja.Value {
			obj := vm.NewObject()
			for k, v := range sctx.Metadata {
				if len(v) > 0 {
					_ = obj.Set(k, v[0])
				}
			}
			return obj
		})
		_ = reqObj.Set("metadata", metadataObj)
	}

	if sctx.Protocol == "graphql" {
		_ = reqObj.Set("graphqlVariables", sctx.GraphQLVariables)
		_ = reqObj.Set("graphqlOperation", sctx.GraphQLOperation)
	}

	pm := getPmObject(vm)
	_ = pm.Set("request", reqObj)
}

func setupPmResponse(vm *goja.Runtime, sctx request.ScriptContext) *goja.Object {
	respObj := vm.NewObject()
	_ = respObj.Set("code", sctx.ResponseStatusCode)
	_ = respObj.Set("status", reasonPhrase(sctx))
	_ = respObj.Set("responseTime", sctx.ResponseTime.Milliseconds())
	_ = respObj.Set("text", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sctx.ResponseBody)
	})
	_ = respObj.Set("json", func(call goja.FunctionCall) goja.Value {
		jsonObj := vm.Get("JSON").ToObject(vm)
		parse, ok := goja.AssertFunction(jsonObj.Get("parse"))
		if !ok {
			return goja.Undefined()
		}
		parsed, err := parse(goja.Undefined(), vm.ToValue(sctx.ResponseBody))
		if err != nil {
			return goja.Undefined()
		}
		return parsed
	})

	respHeaders := vm.NewObject()
	_ = respHeaders.Set("get", func(call goja.FunctionCall) goja.Value {
		if vals := headerValues(sctx.ResponseHeaders, call.Argument(0).String()); len(vals) > 0 {
			return vm.ToValue(vals[0])
		}
		return goja.Undefined()
	})
	_ = respHeaders.Set("has", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(len(headerValues(sctx.ResponseHeaders, call.Argument(0).String())) > 0)
	})
	_ = respObj.Set("headers", respHeaders)

	if sctx.Protocol == "grpc" {
		_ = respObj.Set("statusText", sctx.ResponseStatusText)
		metadataObj := vm.NewObject()
		for k, v := range sctx.ResponseHeaders {
			if len(v) > 0 {
				_ = metadataObj.Set(k, v[0])
			}
		}
		_ = respObj.Set("metadata", metadataObj)
	}

	pm := getPmObject(vm)
	_ = pm.Set("response", respObj)
	return respObj
}

func setupPmTest(vm *goja.Runtime, log *runLog) {
	pm := getPmObject(vm)
	_ = pm.Set("test", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		fn, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			log.test(request.TestResult{Name: name, Passed: false, Error: "second argument must be a function"})
			return goja.Undefined()
		}

		_, err := fn(goja.Undefined())
		if err != nil {
			var jsErr *goja.Exception
			if !errors.As(err, &jsErr) {
				panic(err)
			}
			log.test(request.TestResult{Name: name, Passed: false, Error: testFailureText(jsErr)})
		} else {
			log.test(request.TestResult{Name: name, Passed: true})
		}
		return goja.Undefined()
	})
}

func getPmObject(vm *goja.Runtime) *goja.Object {
	pmVal := vm.Get("pm")
	if pmVal != nil && pmVal != goja.Undefined() && pmVal != goja.Null() {
		if obj, ok := pmVal.(*goja.Object); ok {
			return obj
		}
	}
	pm := vm.NewObject()
	_ = vm.Set("pm", pm)
	return pm
}

// Runs the script on its own goroutine so a run that ignores the interrupt cannot hold the
// caller. On error the caller must not read what the script wrote — an abandoned run keeps writing.
func runWithTimeout(ctx context.Context, vm *goja.Runtime, script string, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("script engine panicked", "panic", r, "stack", string(debug.Stack()))
				done <- fmt.Errorf("script engine failed: %v", r)
			}
		}()
		_, err := vm.RunString(script)
		done <- scriptError(err)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		if _, ok := err.(*goja.InterruptedError); ok {
			if ctx.Err() != nil {
				return fmt.Errorf("cancelled: %w", ctx.Err())
			}
			return timeoutError(timeout)
		}
		return err
	case <-timer.C:
		stopScript(vm, done, "script timeout exceeded")
		return timeoutError(timeout)
	case <-ctx.Done():
		stopScript(vm, done, "context cancelled")
		return fmt.Errorf("cancelled: %w", ctx.Err())
	}
}

func timeoutError(timeout time.Duration) error {
	return &request.ScriptRunError{Message: fmt.Sprintf("Script timed out after %s", timeout), Timeout: true}
}

func stopScript(vm *goja.Runtime, done <-chan error, reason string) {
	vm.Interrupt(reason)

	grace := time.NewTimer(interruptGrace)
	defer grace.Stop()

	select {
	case <-done:
	case <-grace.C:
		abandonedScripts.Add(1)
		slog.Warn("script did not stop on interrupt, left running",
			"reason", reason, "abandoned_total", abandonedScripts.Load())
	}
}

// Runs on the script goroutine: converting a thrown value to text runs script code.
func scriptError(err error) (out error) {
	var interrupted *goja.InterruptedError
	var jsErr *goja.Exception
	switch {
	case err == nil, errors.As(err, &interrupted):
		return err
	case !errors.As(err, &jsErr):
		return fmt.Errorf("script error: %w", err)
	}
	defer func() {
		switch r := recover().(type) {
		case nil:
		case *goja.InterruptedError:
			out = r
		default:
			out = errors.New("script error: the thrown value could not be converted to text")
		}
	}()
	return runError(jsErr)
}

var syntaxErrorText = regexp.MustCompile(`^(?:SyntaxError: )?\(anonymous\): Line (\d+):(\d+) (.+?)(?: \(and \d+ more errors?\))?$`)

func runError(ex *goja.Exception) *request.ScriptRunError {
	if obj, ok := ex.Value().(*goja.Object); ok {
		if name, msg := obj.Get("name"), obj.Get("message"); name != nil && msg != nil && name.String() == "SyntaxError" {
			if m := syntaxErrorText.FindStringSubmatch(msg.String()); m != nil {
				line, _ := strconv.Atoi(m[1])
				col, _ := strconv.Atoi(m[2])
				return &request.ScriptRunError{Message: "SyntaxError: " + m[3], Line: line, Column: col}
			}
		}
	}
	out := &request.ScriptRunError{Message: ex.Value().String()}
	for _, frame := range ex.Stack() {
		if frame.SrcName() == "" {
			pos := frame.Position()
			out.Line, out.Column = pos.Line, pos.Column
			break
		}
	}
	return out
}

func testFailureText(ex *goja.Exception) string {
	if obj, ok := ex.Value().(*goja.Object); ok {
		if name := obj.Get("name"); name != nil && name.String() == "AssertionError" {
			return obj.String()
		}
	}
	return runError(ex).Error()
}

func headerArg(vm *goja.Runtime, call goja.FunctionCall) (string, string) {
	arg := call.Argument(0)
	if goja.IsUndefined(arg) || goja.IsNull(arg) {
		panic(vm.NewTypeError("pm.request.headers expects { key, value }"))
	}
	obj := arg.ToObject(vm)
	key := obj.Get("key")
	if key == nil || goja.IsUndefined(key) || goja.IsNull(key) {
		panic(vm.NewTypeError("pm.request.headers expects { key, value }"))
	}
	value := obj.Get("value")
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return key.String(), ""
	}
	return key.String(), value.String()
}

func deleteHeader(headers map[string][]string, name string) {
	for k := range headers {
		if strings.EqualFold(k, name) {
			delete(headers, k)
		}
	}
}

func headerValues(headers map[string][]string, name string) []string {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return nil
}

func copyMap(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// Always returns a usable map: pm.request.headers.upsert and metadata.set write into it.
func copyHeaders(h map[string][]string) map[string][]string {
	cp := make(map[string][]string, len(h))
	for k, v := range h {
		vals := make([]string, len(v))
		copy(vals, v)
		cp[k] = vals
	}
	return cp
}
