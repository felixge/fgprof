// Package gprof implements an experimental goroutine profiler that allows
// users to analyze function time spent On-CPU as well as Off-CPU [1] (e.g.
// waiting for I/O) together. This does not seem to be possible with the
// builtin Go tools.
//
// [1] http://www.brendangregg.com/offcpuanalysis.html
package gprof

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Start begins profiling the goroutines of the program and returns a function
// that needs to be invoked by the caller to stop the profiling and write the
// results to w. The results are written in the folded stack format used by
// Brendan Gregg's FlameGraph utility [1].
//
// [1] https://github.com/brendangregg/FlameGraph#2-fold-stacks
func Start(w io.Writer) func() error {
	// Go's CPU profiler uses 100hz, but 99hz might be less likely to result in
	// accidental synchronization with the program we're profiling.
	const hz = 99
	ticker := time.NewTicker(time.Second / hz)
	stopCh := make(chan struct{})

	stackCounts := stackCounter{}
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stackCounts.Update()
			case <-stopCh:
				return
			}
		}
	}()

	return func() error {
		stopCh <- struct{}{}

		// Sort the stacks since I suspect that Brendan Gregg's FlameGraph tool's
		// display order is influenced by it.
		var stacks []string
		for stack := range stackCounts {
			stacks = append(stacks, stack)
		}
		sort.Strings(stacks)

		for _, stack := range stacks {
			count := stackCounts[stack]
			if _, err := fmt.Fprintf(w, "%s %d\n", stack, count); err != nil {
				return err
			}
		}

		return nil
	}
}

type stackCounter map[string]int

func (s stackCounter) Update() {
	// Determine the runtime.Frame of this func so we can hide it from our
	// profiling output.
	rpc := make([]uintptr, 1)
	n := runtime.Callers(1, rpc)
	if n < 1 {
		panic("bad")
	}
	selfFrame, _ := runtime.CallersFrames(rpc).Next()

	// COPYRIGHT: The code for populating `p` below is copied from
	// writeRuntimeProfile in src/runtime/pprof/pprof.go.
	//
	// Find out how many records there are (GoroutineProfile(nil)),
	// allocate that many records, and get the data.
	// There's a race—more records might be added between
	// the two calls—so allocate a few extra records for safety
	// and also try again if we're very unlucky.
	// The loop should only execute one iteration in the common case.
	var p []runtime.StackRecord
	n, ok := runtime.GoroutineProfile(nil)
	for {
		// Allocate room for a slightly bigger profile,
		// in case a few more entries have been added
		// since the call to ThreadProfile.
		p = make([]runtime.StackRecord, n+10)
		n, ok = runtime.GoroutineProfile(p)
		if ok {
			p = p[0:n]
			break
		}
		// Profile grew; try again.
	}

outer:
	for _, pp := range p {
		frames := runtime.CallersFrames(pp.Stack())

		var stack []string
		for {
			frame, more := frames.Next()
			if !more {
				break
			} else if frame.Entry == selfFrame.Entry {
				continue outer
			}

			stack = append([]string{frame.Function}, stack...)
		}
		key := strings.Join(stack, ";")
		s[key]++
	}
}

// Handler returns an http handler that takes a "?seconds=N" query argument
// which produces a goroutine profile over the given duration in a text format
// that can be visualized with Brendan Gregg's FlameExplain tool.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var seconds int
		if _, err := fmt.Sscanf(r.URL.Query().Get("seconds"), "%d", &seconds); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "bad seconds: %d: %s\n", seconds, err)
		}

		stop := Start(w)
		defer stop()
		time.Sleep(time.Duration(seconds) * time.Second)
	})
}
