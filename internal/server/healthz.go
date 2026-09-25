package server

import "net/http"

// handleHealthz answers 200 once the server is listening. Tests and
// `make demo` wait on it.
func handleHealthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
}
