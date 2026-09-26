package triage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/redscaresu/itsm/internal/domain"
)

// judgment builds a run with the three severity inputs set.
func judgment(kind domain.Kind, scope domain.Scope, blocked float64) domain.Judgment {
	return domain.Judgment{
		Kind:    domain.Choice{Option: string(kind), Confidence: 1},
		Scope:   domain.Choice{Option: string(scope), Confidence: 1},
		Blocked: blocked,
	}
}

// Catches: someone reorders or edits a section 5 row, or moves a threshold
// boundary, and a ticket gets the wrong severity or rule string.
func TestSeverity(t *testing.T) {
	cases := []struct {
		name string
		j    domain.Judgment
		want SeverityResult
	}{
		{"row 1 change beats national", judgment(domain.KindChange, domain.ScopeNational, 0.99),
			SeverityResult{domain.Sev4, RuleChangeOrQuestion, false}},
		{"row 1 question", judgment(domain.KindQuestion, domain.ScopeOneUser, 0.99),
			SeverityResult{domain.Sev4, RuleChangeOrQuestion, false}},
		{"row 2 national", judgment(domain.KindFault, domain.ScopeNational, 0),
			SeverityResult{domain.Sev1, RuleNational, false}},
		{"row 3 region", judgment(domain.KindFault, domain.ScopeRegion, 0),
			SeverityResult{domain.Sev2, RuleOneOfficeRegion, false}},
		{"row 3 one office", judgment(domain.KindFault, domain.ScopeOneOffice, 0),
			SeverityResult{domain.Sev2, RuleOneOfficeRegion, false}},
		{"row 4 one user blocked at NoulYes", judgment(domain.KindFault, domain.ScopeOneUser, 0.50),
			SeverityResult{domain.Sev3, RuleOneUserBlocked, false}},
		{"row 5 one user just below NoulYes", judgment(domain.KindFault, domain.ScopeOneUser, 0.4999),
			SeverityResult{domain.Sev4, RuleOneUserWorking, false}},
		{"row 6 unclear", judgment(domain.KindFault, domain.ScopeUnclear, 0.99),
			SeverityResult{domain.Sev3, RuleUnclear, false}},
		{"row 7 unknown scope", judgment(domain.KindFault, "one_building", 0.99),
			SeverityResult{domain.Sev3, RuleUnmatched, true}},
		{"row 7 unknown kind is not filed as a change", judgment("incident", domain.ScopeNational, 0.99),
			SeverityResult{domain.Sev3, RuleUnmatched, true}},
		{"row 7 empty answer", domain.Judgment{},
			SeverityResult{domain.Sev3, RuleUnmatched, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Severity(tc.j))
		})
	}
}

// Catches: the team-confidence boundary moves, or an unknown team ID is
// written onto a ticket instead of going to Service Desk for review.
func TestRoute(t *testing.T) {
	cases := []struct {
		name       string
		team       domain.Choice
		wantTeam   domain.Team
		wantReview bool
	}{
		{"confident", domain.Choice{Option: "network", Confidence: 1}, domain.TeamNetwork, false},
		{"at TeamConfidenceMin", domain.Choice{Option: "applications", Confidence: 0.70}, domain.TeamApplications, false},
		{"just below TeamConfidenceMin", domain.Choice{Option: "applications", Confidence: 0.6999}, domain.TeamServiceDesk, true},
		{"unknown team ID", domain.Choice{Option: "facilities", Confidence: 1}, domain.TeamServiceDesk, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			team, review := Route(domain.Judgment{Team: tc.team})
			assert.Equal(t, tc.wantTeam, team)
			assert.Equal(t, tc.wantReview, review)
		})
	}
}
