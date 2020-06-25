package fgprof

import (
	"strings"
	"testing"
)

func Test_toProfile(t *testing.T) {
	s := stackCounter{
		"foo;bar": 2,
		"foo":     1,
	}

	p := toProfile(s, 99)
	if err := p.CheckValid(); err != nil {
		t.Fatal(err)
	}

	want := strings.TrimSpace(`
Period: 0
Samples:
samples/count time/nanoseconds
          1   10101010: 1 
          2   20202020: 2 3 
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
