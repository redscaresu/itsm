package triage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/redscaresu/itsm/internal/domain"
)

// run builds a fault run with the follow-up inputs set.
func run(teamConf float64, scope domain.Scope, loc, svc float64) domain.Judgment {
	j := judgment(domain.KindFault, scope, 0.9)
	j.Team = domain.Choice{Option: string(domain.TeamApplications), Confidence: teamConf}
	j.StatesLocation, j.StatesService = loc, svc
	return j
}

func ids(fs []domain.FollowUp) []domain.FollowUpID {
	var out []domain.FollowUpID
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return out
}

// Catches: someone re-widens the trigger (e.g. "any low states_*") and
// confident tickets such as demo tickets 5 and 6 start asking questions.
func TestFollowUpTriggered(t *testing.T) {
	cases := []struct {
		name string
		j    domain.Judgment
		want bool
	}{
		{"scope unclear", run(1, domain.ScopeUnclear, 1, 1), true},
		{"team conf just below min", run(0.6999, domain.ScopeOneUser, 1, 1), true},
		{"team conf at min", run(0.70, domain.ScopeOneUser, 1, 1), false},
		{"confident but detail missing (ticket 5)", run(1, domain.ScopeOneUser, 0.07, 0.28), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, FollowUpTriggered(tc.j))
		})
	}
}

// Catches: a row's condition, boundary or order changes, so the customer
// is asked the wrong question or the questions appear out of row order.
func TestSelectFollowUps(t *testing.T) {
	cases := []struct {
		name string
		j    domain.Judgment
		want []domain.FollowUpID
	}{
		{"all three rows, in row order (ticket 4)", run(1, domain.ScopeUnclear, 0.03, 0.03),
			[]domain.FollowUpID{domain.FollowUpLocation, domain.FollowUpService, domain.FollowUpImpact}},
		{"row 1 just below DetailMissingMax", run(1, domain.ScopeOneUser, 0.4999, 0.50),
			[]domain.FollowUpID{domain.FollowUpLocation}},
		{"row 2 just below DetailMissingMax", run(1, domain.ScopeOneUser, 0.50, 0.4999),
			[]domain.FollowUpID{domain.FollowUpService}},
		{"row 3 only", run(1, domain.ScopeUnclear, 0.9, 0.9),
			[]domain.FollowUpID{domain.FollowUpImpact}},
		{"nothing missing", run(1, domain.ScopeOneUser, 0.50, 0.50), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ids(SelectFollowUps(tc.j)))
		})
	}
}

// Catches: the pre-written question text drifts from HLD 4.4, or an answer
// is pre-filled.
func TestSelectFollowUpsText(t *testing.T) {
	got := SelectFollowUps(run(1, domain.ScopeUnclear, 0, 0))
	assert.Equal(t, []domain.FollowUp{
		{ID: domain.FollowUpLocation, Question: "Which office or site is affected? If it's everywhere, or just you, say so."},
		{ID: domain.FollowUpService, Question: "Which system, application or device has the problem?"},
		{ID: domain.FollowUpImpact, Question: "Roughly how many people are affected — just you, your office, or more?"},
	}, got)
}

// Catches: a second round sets needs_info (an endless follow-up loop), a
// trigger with nothing to ask leaves the ticket unflagged, or a confident
// ticket is interrupted.
func TestDecide(t *testing.T) {
	cases := []struct {
		name       string
		j          domain.Judgment
		firstRound bool
		wantStatus domain.Status
		wantReview bool
		wantAsked  int
	}{
		{"first round, trigger, rows select", run(1, domain.ScopeUnclear, 0.03, 0.03), true, domain.StatusNeedsInfo, true, 3},
		{"second round never needs_info", run(1, domain.ScopeUnclear, 0.03, 0.03), false, domain.StatusOpen, true, 0},
		{"trigger but no row selects", run(0.5, domain.ScopeOneUser, 0.9, 0.9), true, domain.StatusOpen, true, 0},
		{"no trigger", run(1, domain.ScopeOneUser, 0.07, 0.28), true, domain.StatusOpen, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := Decide(tc.j, tc.firstRound)
			assert.Equal(t, tc.wantStatus, o.Status)
			assert.Equal(t, tc.wantReview, o.NeedsReview)
			assert.Len(t, o.FollowUps, tc.wantAsked)
		})
	}
}

// Catches: a needs_info ticket is left unrouted; HLD 4.4 says it still gets
// a provisional team and severity so nothing sits unowned.
func TestDecideRoutesProvisionally(t *testing.T) {
	j := run(1, domain.ScopeUnclear, 0.03, 0.03)
	j.Team.Option = string(domain.TeamServiceDesk)
	o := Decide(j, true)
	assert.Equal(t, domain.TeamServiceDesk, o.Team)
	assert.Equal(t, domain.Sev3, o.Severity)
	assert.Equal(t, RuleUnclear, o.Rule)
}
