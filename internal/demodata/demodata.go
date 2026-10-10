// Package demodata reads the six demo tickets in testdata/demo/tickets.json
// (HLD section 10). One file feeds the live smoke test's assertions, the
// fake jev script for the end-to-end tests, and the walkthrough's typing.
package demodata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
)

// File is the demo data's path relative to the module root.
const File = "testdata/demo/tickets.json"

// Ticket is one demo ticket.
type Ticket struct {
	N           int      `json:"n"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Expect      Expect   `json:"expect"`
	Answers     *Answers `json:"follow_up_answers,omitempty"` // ticket 4 only
	ExpectAfter *After   `json:"expect_after,omitempty"`      // ticket 4 only
	// Jev is what live jev answered on 2026-09-25 (HLD 4.5), one entry per
	// run. The fake jev replays it; the live test compares against it.
	Jev []Run `json:"jev"`
}

// Expect is the outcome of the first triage run.
type Expect struct {
	Team        domain.Team     `json:"team"`
	Severity    domain.Severity `json:"severity"`
	Status      domain.Status   `json:"status"`
	NeedsReview bool            `json:"needs_review"`
}

// After is the outcome of the second run, once the follow-ups are answered.
type After struct {
	Team     domain.Team     `json:"team"`
	Severity domain.Severity `json:"severity"`
}

// Answers are the customer's follow-up answers, keyed by question (HLD 4.4).
type Answers struct {
	Location string `json:"location"`
	Service  string `json:"service"`
	Impact   string `json:"impact"`
}

// For returns the answer to one follow-up question, or "" for an unknown ID.
func (a Answers) For(id domain.FollowUpID) string {
	switch id {
	case domain.FollowUpLocation:
		return a.Location
	case domain.FollowUpService:
		return a.Service
	case domain.FollowUpImpact:
		return a.Impact
	}
	return ""
}

// Run is jev's answers for one triage run, as in the HLD 4.5 table.
type Run struct {
	Team            domain.Team  `json:"team"`
	TeamConfidence  float64      `json:"team_confidence"`
	Kind            domain.Kind  `json:"kind"`
	Scope           domain.Scope `json:"scope"`
	ScopeConfidence float64      `json:"scope_confidence"`
	Blocked         float64      `json:"blocked"`
	StatesLocation  float64      `json:"states_location"`
	StatesService   float64      `json:"states_service"`
}

// Load reads and checks the demo tickets at path.
func Load(path string) ([]Ticket, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- callers pass the constant File or a test fixture; no request input reaches path
	if err != nil {
		return nil, fmt.Errorf("demodata: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var tickets []Ticket
	if err := dec.Decode(&tickets); err != nil {
		return nil, fmt.Errorf("demodata: decode %s: %w", path, err)
	}
	for _, t := range tickets {
		if err := t.check(); err != nil {
			return nil, fmt.Errorf("demodata: ticket %d: %w", t.N, err)
		}
	}
	return tickets, nil
}

// check rejects a ticket the tests and walkthrough cannot use.
func (t Ticket) check() error {
	switch {
	case t.Title == "" || t.Description == "":
		return fmt.Errorf("title and description are required")
	case len(t.Jev) == 0:
		return fmt.Errorf("no jev answers")
	case (t.Answers == nil) != (t.ExpectAfter == nil):
		return fmt.Errorf("follow_up_answers and expect_after go together")
	case t.Answers != nil && len(t.Jev) < 2:
		return fmt.Errorf("a follow-up round needs jev answers for two runs")
	}
	return nil
}

// Script turns the tickets' jev answers into a fake jev script, keyed on
// title, one entry per run.
func Script(tickets []Ticket) map[string][]jevfake.Answers {
	s := make(map[string][]jevfake.Answers, len(tickets))
	for _, t := range tickets {
		for _, r := range t.Jev {
			s[t.Title] = append(s[t.Title], jevfake.Answers{
				Team:            r.Team,
				TeamConfidence:  r.TeamConfidence,
				Kind:            r.Kind,
				Scope:           r.Scope,
				ScopeConfidence: r.ScopeConfidence,
				Blocked:         r.Blocked,
				StatesLocation:  r.StatesLocation,
				StatesService:   r.StatesService,
			})
		}
	}
	return s
}
