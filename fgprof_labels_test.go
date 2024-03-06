package fgprof

import (
	"bytes"
	"context"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

func Test_Labels(t *testing.T) {
	var buf bytes.Buffer
	stop := Start(&buf)
	go func() {
		fun()
	}()

	time.Sleep(3 * time.Second)
	require.NoError(t, stop())

	p, err := profile.ParseData(buf.Bytes())
	require.NoError(t, err)
	t.Log(len(p.Sample))
	t.Log(p.String())
}

//go:noinline
func work(n int) {
	for i := 0; i < n; i++ {
	}
}

func fastFunction(c context.Context, wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		pprof.Do(c, pprof.Labels("function", "fast"), func(_ context.Context) {
			work(200000000)
		})
	}()
}

func slowFunction(c context.Context, wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		pprof.Do(c, pprof.Labels("function", "slow"), func(c context.Context) {
			work(800000000)
		})
	}()
}

func fun() {
	pprof.Do(context.Background(), pprof.Labels("foo", "bar"), func(ctx context.Context) {
		for {
			wg := sync.WaitGroup{}
			wg.Add(2)
			fastFunction(ctx, &wg)
			slowFunction(ctx, &wg)
			wg.Wait()
		}
	})
}
