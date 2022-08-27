package fgprof

import (
	"strings"
	"testing"
	"time"
)

func Test_toPprof(t *testing.T) {
	s := map[string]int{
		"foo;bar": 2,
		"foo":     1,
	}

	before := time.Local
	defer func() { time.Local = before }()
	time.Local = time.UTC

	start := time.Date(2022, 8, 27, 14, 32, 23, 0, time.UTC)
	end := start.Add(time.Second)
	p := toPprof(s, 99, start, end)
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
          2   20202020: 3 2 
Locations
     1: 0x0 M=1 foo :0 s=0()
     2: 0x0 M=1 foo :0 s=0()
     3: 0x0 M=1 bar :0 s=0()
Mappings
1: 0x0/0x0/0x0   [FN]
`)
	got := strings.TrimSpace(p.String())
	if want != got {
		t.Fatalf("got:\n%s\n\nwant:\n%s", got, want)
	}
}
