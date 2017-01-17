package main

import "net/http"

type Route struct {
	name 	string
	method 	string
	pattern string
	handler http.HandlerFunc
}

type Routes []Route

// Define any application routes here
var routes = Routes {
	Route {
		"Store Items",
		"POST",
		"/items",
		IC.store,
	},
	Route {
		"Create Item",
		"POST",
		"/items/{item_name}",
		IC.fetchOrStore,
	},
}