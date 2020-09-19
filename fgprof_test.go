package fgprof

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestStart is a simple smoke test that checks that the profiler doesn't
// produce errors and catches the TestStart function itself. It'd be nice to
// add better testing in the future, but writing test cases for a profiler is
// a little tricky : ).
func TestStart(t *testing.T) {
	out := &bytes.Buffer{}
	stop := Start(out, FormatFolded)
	time.Sleep(100 * time.Millisecond)
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "fgprof.TestStart") {
		t.Fatalf("invalid output:\n%s", out)
	}
}

func BenchmarkProfiler(b *testing.B) {
	prof := &profiler{}
	for i := 0; i < b.N; i++ {
		prof.GoroutineProfile()
	}
}

func BenchmarkStackCounter(b *testing.B) {
	prof := &profiler{}
	stacks := prof.GoroutineProfile()
	sc := stackCounter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sc.Update(stacks)
	}
}
