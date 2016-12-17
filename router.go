package main

import (
	"github.com/gorilla/mux"
	"net/http"
)

func CreateRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	for _, route := range routes {
		// Decorate the HTTP handler with a server log so we can debug the routes
		var handler http.Handler

		handler = route.handler

		handler = Logger(handler, route.name)

		router.
		Methods(route.method).
			Path(route.pattern).
			Name(route.name).
			Handler(handler)
	}

	return router
}
