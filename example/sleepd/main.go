package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	h := http.HandlerFunc(sleepHandler)
	addr := os.Getenv("SLEEPD_ADDR")
	if addr == "" {
		addr = "localhost:6061"
	}
	log.Println(http.ListenAndServe(addr, h))
}

func sleepHandler(w http.ResponseWriter, r *http.Request) {
	sleep := r.URL.Query().Get("sleep")
	sleepD, err := time.ParseDuration(sleep)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "bad duration: %s: %s\n", sleep, err)
	}
	time.Sleep(sleepD)
	fmt.Fprintf(w, "slept for: %s\n", sleepD)
}
