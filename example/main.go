package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	_ "net/http/pprof"

	"github.com/felixge/fgprof"
)

const (
	sleepTime   = 10 * time.Millisecond
	cpuTime     = 30 * time.Millisecond
	networkTime = 60 * time.Millisecond
)

// sleepURL is the url for the sleep server used by slowNetworkRequest. It's
// a global variable to keep the cute simplicity of main's loop.
var sleepURL string

func main() {
	// Run http endpoints for both pprof and fgprof.
	http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())
	go func() {
		addr := "localhost:6060"
		log.Printf("Listening on %s", addr)
		log.Println(http.ListenAndServe(addr, nil))
	}()

	// Start a sleep server to help with simulating slow network requests.
	var stop func()
	sleepURL, stop = StartSleepServer()
	defer stop()

	for {
		// Http request to a web service that might be slow.
		slowNetworkRequest()
		// Some heavy CPU computation.
		cpuIntensiveTask()
		// Poorly named function that you don't understand yet.
		weirdFunction()
	}
}

func slowNetworkRequest() {
	res, err := http.Get(sleepURL + "/?sleep=" + networkTime.String())
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		panic(fmt.Sprintf("bad code: %d", res.StatusCode))
	}
}

func cpuIntensiveTask() {
	start := time.Now()
	for time.Since(start) <= cpuTime {
		// Spend some time in a hot loop to be a little more realistic than
		// spending all time in time.Since().
		for i := 0; i < 1000; i++ {
			_ = i
		}
	}
}

func weirdFunction() {
	time.Sleep(sleepTime)
}
