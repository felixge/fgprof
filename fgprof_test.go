package fgprof

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

// TestStart is a smoke test that checks that the profiler produces a profiles
// in different formats with the expected stack frames.
func TestStart(t *testing.T) {
	tests := []struct {
		// Format is the export format being tested
		Format Format
		// ContainsStack returns true if the given profile contains a frame with the given name
		ContainsStack func(t *testing.T, prof *bytes.Buffer, frame string) bool
	}{
		{
			Format: FormatFolded,
			ContainsStack: func(t *testing.T, prof *bytes.Buffer, frame string) bool {
				return strings.Contains(prof.String(), frame)
			},
		},
		{
			Format: FormatPprof,
			ContainsStack: func(t *testing.T, prof *bytes.Buffer, frame string) bool {
				pprof, err := profile.ParseData(prof.Bytes())
				require.NoError(t, err)
				require.NoError(t, pprof.CheckValid())
				for _, s := range pprof.Sample {
					for _, loc := range s.Location {
						for _, line := range loc.Line {
							if strings.Contains(line.Function.Name, frame) {
								return true
							}
						}
					}
				}
				return false
			},
		},
	}

	for _, test := range tests {
		t.Run(string(test.Format), func(t *testing.T) {
			prof := &bytes.Buffer{}
			stop := Start(prof, test.Format)
			time.Sleep(100 * time.Millisecond)
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			require.True(t, test.ContainsStack(t, prof, "fgprof.TestStart"))
			require.False(t, test.ContainsStack(t, prof, "GoroutineProfile"))
		})
	}
}

func Test_toPprof(t *testing.T) {
	foo := &runtime.Frame{PC: 1, Function: "foo", File: "foo.go", Line: 23}
	bar := &runtime.Frame{PC: 2, Function: "bar", File: "bar.go", Line: 42}
	prof := &wallclockProfile{
		stacks: map[[32]uintptr]*wallclockStack{
			{foo.PC}: {
				frames: []*runtime.Frame{foo},
				count:  1,
			},
			{bar.PC, foo.PC}: {
				frames: []*runtime.Frame{bar, foo},
				count:  2,
			},
		},
	}

	before := time.Local
	defer func() { time.Local = before }()
	time.Local = time.UTC

	start := time.Date(2022, 8, 27, 14, 32, 23, 0, time.UTC)
	end := start.Add(time.Second)
	p := prof.exportPprof(99, start, end)
	if err := p.CheckValid(); err != nil {
		t.Fatal(err)
	}

	want := strings.TrimSpace(`
PeriodType: wallclock nanoseconds
Period: 10101010
Time: 2022-08-27 14:32:23 +0000 UTC
Duration: 1s
Samples:
samples/count time/nanoseconds
          1   10101010: 1 
          2   20202020: 2 1 
Locations
     1: 0x0 M=1 foo foo.go:23 s=0
     2: 0x0 M=1 bar bar.go:42 s=0
Mappings
1: 0x0/0x0/0x0   [FN]
`)
	got := strings.TrimSpace(p.String())
	require.Equal(t, got, want)
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
	sc := wallclockProfile{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sc.Add(stacks)
	}
}
