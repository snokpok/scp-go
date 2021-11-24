package utils

import "net/http"

func MiddlewaresWrapper(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// h will be executed first, then the middlewares from last to first
	for _, mw := range middlewares {
		h = mw(h)
	}
	return h
}
