package fgprof

import (
	"fmt"
	"net/http"
	"time"
)

// Handler returns an http handler that requires a "seconds" query argument
// and produces a profile over this duration. The optional "format" parameter
// controls if the output is written in Google's "pprof" format (default) or
// Brendan Gregg's "folded" stack format.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var seconds int
		if _, err := fmt.Sscanf(r.URL.Query().Get("seconds"), "%d", &seconds); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "bad seconds: %d: %s\n", seconds, err)
		}

		format := Format(r.URL.Query().Get("format"))
		if format == "" {
			format = FormatPprof
		}

		stop := Start(w, format)
		defer stop()
		time.Sleep(time.Duration(seconds) * time.Second)
	})
}
