package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/felixge/gprof/example"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	example.Program()
}
