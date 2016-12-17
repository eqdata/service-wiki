package main

import (
	"log"
	"net/http"
	"time"
)

// Logs the time in which we resolve the HTTP request handler func on the server,
// this will allow us to measure how fast we are serving requests and discover
// and potential bottle necks.
func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		inner.ServeHTTP(w, r)

		log.Printf(
			"%s\t%s\t%s\t%s",
			r.Method,
			r.RequestURI,
			name,
			time.Since(start),
		)
	})
}