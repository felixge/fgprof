package fgprof

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

func Test_toProfile(t *testing.T) {
	s := stackCounter{
		"foo;bar": 2,
		"foo":     1,
	}

	p := toProfile(s, 99)
	c := qt.New(t)
	c.Assert(p.String(), qt.Equals, p.String())
	c.Assert(p.CheckValid(), qt.IsNil)

	file, err := os.Create("test.profile")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	p.WriteUncompressed(file)
}
