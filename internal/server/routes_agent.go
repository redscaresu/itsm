package server

import "net/http"

// addAgentRoutes registers the agent screen (HLD 3.2).
func addAgentRoutes(mux *http.ServeMux, d deps) {
	mux.Handle("GET /agent", handleAgentQueuePage(d))
	mux.Handle("GET /agent/queue", handleAgentQueue(d))
	mux.Handle("GET /agent/feed", handleAgentFeed(d))
	mux.Handle("GET /agent/tickets/{id}", handleAgentTicket(d))
	mux.Handle("POST /agent/tickets/{id}/reassign", handleAgentReassign(d))
	mux.Handle("POST /agent/tickets/{id}/severity", handleAgentSeverity(d))
	mux.Handle("POST /agent/tickets/{id}/close", handleAgentClose(d))
	mux.Handle("POST /agent/tickets/{id}/reviewed", handleAgentReviewed(d))
}
