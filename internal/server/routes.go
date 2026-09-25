package server

import (
	"net/http"

	"github.com/redscaresu/itsm/internal/web"
)

// addRoutes is the single index of routes. The customer and agent screens
// live in their own files so they can be built in parallel (HLD section 7).
func addRoutes(mux *http.ServeMux, d deps) {
	mux.Handle("GET /healthz", handleHealthz())
	// govuk-frontend's CSS hard-codes /assets/fonts/… and /assets/images/….
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(web.Assets())))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(web.Static())))
	addCustomerRoutes(mux, d)
	addAgentRoutes(mux, d)
}
