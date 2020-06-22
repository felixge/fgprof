package main

import (
	"os"
	"time"

	"github.com/felixge/gprof"
)

func main() {
	stop := gprof.Start(os.Stdout)
	defer stop()

	for i := 0; i < 100; i++ {
		waitForNetwork(75 * time.Millisecond)
		keepCPUBusy(25 * time.Millisecond)
	}
}

func waitForNetwork(d time.Duration) {
	time.Sleep(d)
}

func keepCPUBusy(d time.Duration) {
	start := time.Now()
	for time.Since(start) < d {
	}
}
