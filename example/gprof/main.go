package main

import (
	"log"
	"net/http"

	"github.com/felixge/gprof"
	"github.com/felixge/gprof/example"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", gprof.Handler()))
	}()

	example.Program()
}
