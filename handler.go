package fgprof

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Handler returns an http handler that requires a "seconds" query argument
// and produces a profile over this duration. The optional "format" parameter
// controls if the output is written in Brendan Gregg's "folded" stack
// format, or Google's "pprof" format. If no "format" is given, the handler
// tries to guess the best format based on the http headers.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var seconds int
		if _, err := fmt.Sscanf(r.URL.Query().Get("seconds"), "%d", &seconds); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "bad seconds: %d: %s\n", seconds, err)
		}

		format := Format(r.URL.Query().Get("format"))
		if format == "" {
			format = guessFormat(r)
		}

		stop := Start(w, format)
		defer stop()
		time.Sleep(time.Duration(seconds) * time.Second)
	})
}

// guessFormat returns FormatPprof if it looks like pprof sent r, otherwise
// FormatFolded. It's not meant to be a perfect heuristic, but a nice
// convenience for users that don't want to specify the format explicitly.
func guessFormat(r *http.Request) Format {
	for _, format := range r.Header["Accept-Encoding"] {
		if strings.ToLower(format) == "gzip" {
			return FormatPprof
		}
	}
	return FormatFolded
}
