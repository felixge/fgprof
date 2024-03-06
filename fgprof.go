package fgprof

import (
	"io"
	"math"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/google/pprof/profile"
)

func Start(w io.Writer) func() error {
	startTime := time.Now()

	// Go's CPU profiler uses 100hz, but 99hz might be less likely to result in
	// accidental synchronization with the program we're profiling.
	const hz = 99
	ticker := time.NewTicker(time.Second / hz)
	stopCh := make(chan struct{})
	prof := &profiler{}
	p := newWallClockProfile()

	var sampleCount int64

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sampleCount++
				stacks, labels := prof.GoroutineProfile()
				p.Add(stacks, labels)
			case <-stopCh:
				return
			}
		}
	}()

	return func() error {
		stopCh <- struct{}{}
		endTime := time.Now()
		p.Ignore(prof.SelfFrames()...)

		// Compute actual sample rate in case, due to performance issues, we
		// were not actually able to sample at the given hz. Converting
		// everything to float avoids integers being rounded in the wrong
		// direction and improves the correctness of times in profiles.
		duration := endTime.Sub(startTime)
		actualHz := float64(sampleCount) / (float64(duration) / 1e9)
		return p.exportPprof(int64(math.Round(actualHz)), startTime, endTime).Write(w)
	}
}

// profiler provides a convenient and performant way to access
// runtime.GoroutineProfile().
type profiler struct {
	stacks    []runtime.StackRecord
	labels    []unsafe.Pointer
	selfFrame *runtime.Frame
}

//go:linkname runtime_goroutineProfileWithLabels runtime/pprof.runtime_goroutineProfileWithLabels
func runtime_goroutineProfileWithLabels([]runtime.StackRecord, []unsafe.Pointer) (int, bool)

// GoroutineProfile returns the stacks of all goroutines currently managed by
// the scheduler. This includes both goroutines that are currently running
// (On-CPU), as well as waiting (Off-CPU).
func (p *profiler) GoroutineProfile() ([]runtime.StackRecord, []unsafe.Pointer) {
	if p.selfFrame == nil {
		// Determine the runtime.Frame of this func so we can hide it from our
		// profiling output.
		rpc := make([]uintptr, 1)
		n := runtime.Callers(1, rpc)
		if n < 1 {
			panic("could not determine selfFrame")
		}
		selfFrame, _ := runtime.CallersFrames(rpc).Next()
		p.selfFrame = &selfFrame
	}

	// We don't know how many goroutines exist, so we have to grow p.stacks
	// dynamically. We overshoot by 10% since it's possible that more goroutines
	// are launched in between two calls to GoroutineProfile. Once p.stacks
	// reaches the maximum number of goroutines used by the program, it will get
	// reused indefinitely, eliminating GoroutineProfile calls and allocations.
	//
	// TODO(fg) There might be workloads where it would be nice to shrink
	// p.stacks dynamically as well, but let's not over-engineer this until we
	// understand those cases better.
	for {
		n, ok := runtime_goroutineProfileWithLabels(p.stacks, p.labels)
		if !ok {
			p.stacks = make([]runtime.StackRecord, int(float64(n)*1.1))
			p.labels = make([]unsafe.Pointer, int(float64(n)*1.1))
		} else {
			return p.stacks[0:n], p.labels[0:n]
		}
	}
}

// SelfFrames returns frames that belong to the profiler so that we can ignore
// them when exporting the final profile.
func (p *profiler) SelfFrames() []*runtime.Frame {
	if p.selfFrame != nil {
		return []*runtime.Frame{p.selfFrame}
	}
	return nil
}

func newWallClockProfile() *wallClockProfile {
	return &wallClockProfile{stacks: map[[32]uintptr]*stack{}}
}

type wallClockProfile struct {
	stacks map[[32]uintptr]*stack
	ignore []*runtime.Frame
}

type stack struct {
	frames []*runtime.Frame
	dims   []*dimension
}

type dimension struct {
	labels unsafe.Pointer
	key    string
	value  int64
}

func (d *dimension) buildKey(tmp []string) []string {
	if d.labels == nil || d.key != "" {
		return tmp
	}
	d.key, tmp = mapToString(tmp, *(*map[string]string)(d.labels))
	return tmp
}

func mapToString(keys []string, m map[string]string) (string, []string) {
	keys = keys[:0]
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var s strings.Builder
	for _, key := range keys {
		s.WriteString(key)
		s.WriteRune(':')
		s.WriteString(m[key])
		s.WriteRune(';')
	}
	return s.String(), keys
}

func (s *stack) mergeDimensions() {
	tmp := make([]string, 0, 8)
	for _, d := range s.dims {
		tmp = d.buildKey(tmp)
	}
	slices.SortFunc(s.dims, func(a, b *dimension) int {
		return strings.Compare(a.key, b.key)
	})
	s.dims = slices.CompactFunc(s.dims, func(a, b *dimension) bool {
		return a.key == b.key
	})
}

func (s *stack) add(labels unsafe.Pointer) {
	if labels == nil {
		// Fast path: labels are not used.
		if len(s.dims) == 0 {
			s.dims = append(s.dims, &dimension{value: 1})
			return
		}
		if len(s.dims) == 1 && s.dims[0].labels == nil {
			s.dims[0].value++
			return
		}
		// There are multiple dimensions, we need to find
		// one without labels or create it.
		var found bool
		for j := range s.dims {
			if s.dims[j].labels == nil {
				s.dims[j].value++
				found = true
				break
			}
		}
		if !found {
			s.dims = append(s.dims, &dimension{value: 1})
			return
		}
	}
	// Fast path: assume the pointer only changes when the label
	// set changes, therefore we could compare the pointers.
	var found bool
	for j := range s.dims {
		if s.dims[j].labels == labels {
			s.dims[j].value++
			found = true
			break
		}
	}
	if found {
		return
	}
	// We just append it. Dimensions are to be merged at export.
	s.dims = append(s.dims, &dimension{
		labels: labels,
		value:  1,
	})
}

// Ignore sets a list of frames that should be ignored when exporting the profile.
func (p *wallClockProfile) Ignore(frames ...*runtime.Frame) {
	p.ignore = frames
}

// Add adds the given stack traces to the profile.
func (p *wallClockProfile) Add(stackRecords []runtime.StackRecord, labelRecords []unsafe.Pointer) {
	for i, stackRecord := range stackRecords {
		ws, ok := p.stacks[stackRecord.Stack0]
		if !ok {
			ws = &stack{}
			// symbolize pcs into frames
			frames := runtime.CallersFrames(stackRecord.Stack())
			for {
				frame, more := frames.Next()
				ws.frames = append(ws.frames, &frame)
				if !more {
					break
				}
			}
			p.stacks[stackRecord.Stack0] = ws
		}
		ws.add(labelRecords[i])
	}
}

// exportStacks returns the stacks in this profile except those that have been
// set to Ignore().
func (p *wallClockProfile) exportStacks() []*stack {
	stacks := make([]*stack, 0, len(p.stacks))
nextStack:
	for _, ws := range p.stacks {
		for _, f := range ws.frames {
			for _, igf := range p.ignore {
				if f.Entry == igf.Entry {
					continue nextStack
				}
			}
		}
		stacks = append(stacks, ws)
	}
	return stacks
}

func (p *wallClockProfile) exportPprof(hz int64, startTime, endTime time.Time) *profile.Profile {
	prof := &profile.Profile{}
	m := &profile.Mapping{ID: 1, HasFunctions: true}
	prof.Period = int64(1e9 / hz) // Number of nanoseconds between samples.
	prof.TimeNanos = startTime.UnixNano()
	prof.DurationNanos = int64(endTime.Sub(startTime))
	prof.Mapping = []*profile.Mapping{m}
	prof.SampleType = []*profile.ValueType{
		{
			Type: "wall",
			Unit: "nanoseconds",
		},
	}
	prof.PeriodType = &profile.ValueType{
		Type: "wall",
		Unit: "nanoseconds",
	}

	type functionKey struct {
		Name     string
		Filename string
	}
	funcIdx := map[functionKey]*profile.Function{}

	type locationKey struct {
		Function functionKey
		Line     int
	}
	locationIdx := map[locationKey]*profile.Location{}
	for _, ws := range p.exportStacks() {
		locs := make([]*profile.Location, 0, 32)
		for _, frame := range ws.frames {
			fnKey := functionKey{Name: frame.Function, Filename: frame.File}
			function, ok := funcIdx[fnKey]
			if !ok {
				function = &profile.Function{
					ID:         uint64(len(prof.Function)) + 1,
					Name:       frame.Function,
					SystemName: frame.Function,
					Filename:   frame.File,
				}
				funcIdx[fnKey] = function
				prof.Function = append(prof.Function, function)
			}

			locKey := locationKey{Function: fnKey, Line: frame.Line}
			location, ok := locationIdx[locKey]
			if !ok {
				location = &profile.Location{
					ID:      uint64(len(prof.Location)) + 1,
					Mapping: m,
					Line: []profile.Line{{
						Function: function,
						Line:     int64(frame.Line),
					}},
				}
				locationIdx[locKey] = location
				prof.Location = append(prof.Location, location)
			}
			locs = append(locs, location)
		}
		ws.mergeDimensions()
		// We reuse the same locs slice as we assume the profile is immutable.
		for i := 0; i < len(ws.dims); i++ {
			prof.Sample = append(prof.Sample, &profile.Sample{
				Location: locs,
				Value:    []int64{1e9 / hz * ws.dims[i].value},
				Label:    convertLabels(ws.dims[i].labels),
			})
		}
	}
	return prof
}

func convertLabels(l unsafe.Pointer) map[string][]string {
	if l == nil {
		return nil
	}
	m := make(map[string][]string)
	s := (*map[string]string)(l)
	for k, v := range *s {
		m[k] = []string{v}
	}
	return m
}
