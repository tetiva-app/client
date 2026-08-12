//go:build sandbox_probe

// Measurement harness for the script sandbox. Not part of the normal test run: these probes
// allocate gigabytes and one of them never finishes on its own.
//
//	go test -tags sandbox_probe -v -timeout 120s ./internal/infrastructure/scriptengine/
package scriptengine_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/infrastructure/scriptengine"
)

func TestProbeMemoryGrowth(t *testing.T) {
	engine := scriptengine.NewGojaEngine()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := engine.RunPreScript(ctx, `var a = []; while (true) { a.push("x".repeat(1024)) }`, newContext())
	elapsed := time.Since(start)

	runtime.ReadMemStats(&after)
	grew := int64(after.HeapAlloc) - int64(before.HeapAlloc)

	t.Logf("err=%v elapsed=%s heap +%s (~%s/s)", err, elapsed.Round(10*time.Millisecond),
		humanBytes(grew), humanBytes(int64(float64(grew)/elapsed.Seconds())))
}

func TestProbeRunCost(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	const runs = 200
	script := `pm.environment.set("k", "v"); pm.request.headers.upsert({key: "X-Sig", value: "abc"})`

	start := time.Now()
	for range runs {
		if _, err := engine.RunPreScript(context.Background(), script, newContext()); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	}
	t.Logf("new VM + setup + typical script: %s per run", (time.Since(start) / runs).Round(time.Microsecond))
}

func TestProbeRegexpEngines(t *testing.T) {
	cases := []struct{ name, pattern string }{
		{"nested-quantifier", `/(a+)+$/`},
		{"backreference", `/(a+)+\1$/`},
		{"lookahead", `/^(?=(a+)+$)a*$/`},
	}

	engine := scriptengine.NewGojaEngine()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := fmt.Sprintf(`var s = "a".repeat(40) + "!"; %s.test(s)`, tc.pattern)
			start := time.Now()
			_, err := engine.RunPreScript(context.Background(), script, newContext())
			t.Logf("%-18s err=%v elapsed=%s", tc.name, err, time.Since(start).Round(10*time.Millisecond))
		})
	}
}

func TestProbeGlobals(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	res, err := engine.RunPreScript(context.Background(),
		`console.log(Object.getOwnPropertyNames(globalThis).length + ": " +
			Object.getOwnPropertyNames(globalThis).sort().join(","))`,
		newContext())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	for _, line := range res.ConsoleOutput {
		t.Log(line)
	}
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}
