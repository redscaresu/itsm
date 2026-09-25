package web

import (
	"html/template"
	"strings"
)

// data-testid values shared by both screens (HLD 3.3). Derived from the
// section 10 scenes: the walkthrough reads a ticket's number, team, severity,
// status and needs-review badge on the customer screen and in the agent queue.
// S4 adds testids_customer.go and S5 adds testids_agent.go; neither edits
// this file.
const (
	TestIDTicketID       = "ticket-id"
	TestIDTicketTeam     = "ticket-team"
	TestIDTicketSeverity = "ticket-severity"
	TestIDTicketStatus   = "ticket-status"
	TestIDNeedsReview    = "needs-review"
)

// testIDFuncs holds one template function per registered test ID. A template
// writes data-testid="{{tid_ticket_team}}" for TestIDTicketTeam: the ID with
// "-" turned into "_" and a "tid_" prefix. Because the function exists only
// if the ID is registered, a template that names a missing or renamed ID
// fails to parse, and the template parse test goes red.
var testIDFuncs = template.FuncMap{}

// registerTestIDs adds a template function for each ID. Each testids*.go
// file calls it once from a package-level var, e.g.
//
//	var _ = registerTestIDs(TestIDCustomerForm, TestIDCustomerSubmit)
//
// It panics on a duplicate, so two files cannot claim one ID.
func registerTestIDs(ids ...string) bool {
	for _, id := range ids {
		name := testIDFuncName(id)
		if _, dup := testIDFuncs[name]; dup {
			panic("web: test ID registered twice: " + id)
		}
		testIDFuncs[name] = func() string { return id }
	}
	return true
}

func testIDFuncName(id string) string {
	return "tid_" + strings.ReplaceAll(id, "-", "_")
}

var _ = registerTestIDs(
	TestIDTicketID,
	TestIDTicketTeam,
	TestIDTicketSeverity,
	TestIDTicketStatus,
	TestIDNeedsReview,
)
