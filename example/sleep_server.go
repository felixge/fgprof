package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

// StartSleepServer starts a server that supports a ?sleep parameter to
// simulate slow http responses. It returns the url of that server and a
// function to stop it.
func StartSleepServer() (url string, stop func()) {
	server := httptest.NewServer(http.HandlerFunc(sleepHandler))
	return server.URL, server.Close
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
