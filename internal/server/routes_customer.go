package server

import "net/http"

// addCustomerRoutes registers the customer screen (HLD 3.1).
func addCustomerRoutes(mux *http.ServeMux, d deps) {
	mux.Handle("GET /{$}", handleCustomerHome(d))
	mux.Handle("POST /tickets", handleTicketCreate(d))
	mux.Handle("GET /tickets/{id}", handleTicketPage(d))
	mux.Handle("GET /tickets/{id}/status", handleTicketStatus(d))
	mux.Handle("POST /tickets/{id}/follow-up", handleFollowUp(d))
}
