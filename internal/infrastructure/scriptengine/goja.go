package scriptengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const (
	scriptTimeout = 5 * time.Second
	// Interrupt is checked between bytecode instructions, so it never lands inside Go code —
	// a regexp with backtracking is the known case. After this grace period we stop waiting.
	interruptGrace = 250 * time.Millisecond
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
	headers := copyHeaders(sctx.RequestHeaders)
	sctx.Metadata = copyHeaders(sctx.Metadata)

	var consoleOutput []string

	setupConsole(vm, &consoleOutput)
	setupPmEnvironment(vm, vars)
	setupPmRequestPreScript(vm, sctx, headers)
	setupPmExpect(vm, nil, false)

	if err := runWithTimeout(ctx, vm, script, scriptTimeout); err != nil {
		return nil, err
	}

	return &request.PreScriptResult{
		Headers:       headers,
		Metadata:      sctx.Metadata,
		Variables:     vars,
		ConsoleOutput: consoleOutput,
	}, nil
}

func (e *GojaEngine) RunPostScript(ctx context.Context, script string, sctx request.ScriptContext) (*request.PostScriptResult, error) {
	vm := goja.New()

	vars := copyMap(sctx.Variables)
	headers := copyHeaders(sctx.RequestHeaders)
	sctx.Metadata = copyHeaders(sctx.Metadata)

	var consoleOutput []string
	var testResults []request.TestResult

	setupConsole(vm, &consoleOutput)
	setupPmEnvironment(vm, vars)
	setupPmRequestPreScript(vm, sctx, headers)
	setupPmExpect(vm, setupPmResponse(vm, sctx), sctx.Protocol == "grpc")
	setupPmTest(vm, &testResults)

	if err := runWithTimeout(ctx, vm, script, scriptTimeout); err != nil {
		return nil, err
	}

	return &request.PostScriptResult{
		TestResults:   testResults,
		Variables:     vars,
		ConsoleOutput: consoleOutput,
	}, nil
}

func setupConsole(vm *goja.Runtime, output *[]string) {
	logFn := func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		*output = append(*output, strings.Join(parts, " "))
		return goja.Undefined()
	}

	console := vm.NewObject()
	_ = console.Set("log", logFn)
	_ = console.Set("warn", logFn)
	_ = console.Set("error", logFn)
	_ = console.Set("info", logFn)
	_ = vm.Set("console", console)
}

func setupPmEnvironment(vm *goja.Runtime, vars map[string]string) {
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

	pm := getPmObject(vm)
	_ = pm.Set("environment", env)
	_ = pm.Set("collectionVariables", env)
}

func setupPmRequestPreScript(vm *goja.Runtime, sctx request.ScriptContext, headers map[string][]string) {
	reqHeaders := vm.NewObject()
	_ = reqHeaders.Set("upsert", func(call goja.FunctionCall) goja.Value {
		obj := call.Argument(0).ToObject(vm)
		key := obj.Get("key").String()
		value := obj.Get("value").String()
		headers[key] = []string{value}
		return goja.Undefined()
	})
	_ = reqHeaders.Set("remove", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		delete(headers, key)
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

func setupPmTest(vm *goja.Runtime, results *[]request.TestResult) {
	pm := getPmObject(vm)
	_ = pm.Set("test", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		fn, ok := goja.AssertFunction(call.Argument(1))
		if !ok {
			*results = append(*results, request.TestResult{
				Name: name, Passed: false, Error: "second argument must be a function",
			})
			return goja.Undefined()
		}

		_, err := fn(goja.Undefined())
		if err != nil {
			var jsErr *goja.Exception
			if !errors.As(err, &jsErr) {
				panic(err)
			}
			*results = append(*results, request.TestResult{
				Name: name, Passed: false, Error: testFailureText(jsErr),
			})
		} else {
			*results = append(*results, request.TestResult{
				Name: name, Passed: true,
			})
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
		_, err := vm.RunString(script)
		done <- scriptError(err)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		if interrupted, ok := err.(*goja.InterruptedError); ok {
			if ctx.Err() != nil {
				return fmt.Errorf("cancelled: %w", ctx.Err())
			}
			return fmt.Errorf("timeout: %s", interrupted.Value())
		}
		return err
	case <-timer.C:
		stopScript(vm, done, "script timeout exceeded")
		return fmt.Errorf("timeout: script timeout exceeded")
	case <-ctx.Done():
		stopScript(vm, done, "context cancelled")
		return fmt.Errorf("cancelled: %w", ctx.Err())
	}
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
	return fmt.Errorf("script error: %s", exceptionText(jsErr))
}

func exceptionText(ex *goja.Exception) string {
	var b bytes.Buffer
	b.WriteString(ex.Value().String())
	for _, frame := range ex.Stack() {
		if frame.SrcName() == "" {
			b.WriteString(" at ")
			frame.Write(&b)
			break
		}
	}
	return b.String()
}

func testFailureText(ex *goja.Exception) string {
	if obj, ok := ex.Value().(*goja.Object); ok {
		if name := obj.Get("name"); name != nil && name.String() == "AssertionError" {
			return obj.String()
		}
	}
	return exceptionText(ex)
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
