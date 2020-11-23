package fgprof

import (
	"bytes"
	"fmt"
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

func BenchmarkProfilerGoroutines(b *testing.B) {
	for g := 1; g <= 1024*1024; g = g * 2 {
		g := g
		name := fmt.Sprintf("%d goroutines", g)

		b.Run(name, func(b *testing.B) {
			prof := &profiler{}
			initalRoutines := len(prof.GoroutineProfile())

			readyCh := make(chan struct{})
			stopCh := make(chan struct{})
			for i := 0; i < g; i++ {
				go func() {
					defer func() { stopCh <- struct{}{} }()
					readyCh <- struct{}{}
				}()
				<-readyCh
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				stacks := prof.GoroutineProfile()
				gotRoutines := len(stacks) - initalRoutines
				if gotRoutines != g {
					b.Logf("want %d goroutines, but got %d on iteration %d", g, len(stacks), i)
				}
			}
			b.StopTimer()
			for i := 0; i < g; i++ {
				<-stopCh
			}
			start := time.Now()
			for i := 0; ; i++ {
				if len(prof.GoroutineProfile()) == initalRoutines {
					break
				}
				time.Sleep(20 * time.Millisecond)
				if time.Since(start) > 10*time.Second {
					b.Fatalf("%d goroutines still running, want %d", len(prof.GoroutineProfile()), initalRoutines)
				}
			}
		})
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
