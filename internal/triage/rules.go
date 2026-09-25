package triage

import "github.com/redscaresu/itsm/internal/domain"

// Rule strings (HLD section 5). Stored on each run as Judgment.Rule and shown
// in the agent's Why panel.
const (
	RuleChangeOrQuestion = "change-or-question"
	RuleNational         = "national"
	RuleOneOfficeRegion  = "one-office-or-region"
	RuleOneUserBlocked   = "one-user-blocked"
	RuleOneUserWorking   = "one-user-working"
	RuleUnclear          = "unclear"
	RuleUnmatched        = "unmatched"
)

// SeverityResult is what the section 5 table gives one run.
type SeverityResult struct {
	Severity    domain.Severity
	Rule        string
	NeedsReview bool
}

// Severity applies the HLD section 5 table, top to bottom, first match wins.
//
// Rule 1 matches the kinds the code knows are not faults (change, question),
// not "anything but fault": an option ID the code does not know must reach
// rule 7 and be flagged, not be silently filed as Sev 4.
func Severity(j domain.Judgment) SeverityResult {
	kind := domain.Kind(j.Kind.Option)
	scope := domain.Scope(j.Scope.Option)
	switch {
	case kind == domain.KindChange || kind == domain.KindQuestion:
		return SeverityResult{Severity: domain.Sev4, Rule: RuleChangeOrQuestion}
	case kind != domain.KindFault:
		// Unknown kind: falls out of the switch to rule 7.
	case scope == domain.ScopeNational:
		return SeverityResult{Severity: domain.Sev1, Rule: RuleNational}
	case scope == domain.ScopeRegion || scope == domain.ScopeOneOffice:
		return SeverityResult{Severity: domain.Sev2, Rule: RuleOneOfficeRegion}
	case scope == domain.ScopeOneUser && j.Blocked >= NoulYes:
		return SeverityResult{Severity: domain.Sev3, Rule: RuleOneUserBlocked}
	case scope == domain.ScopeOneUser:
		return SeverityResult{Severity: domain.Sev4, Rule: RuleOneUserWorking}
	case scope == domain.ScopeUnclear:
		return SeverityResult{Severity: domain.Sev3, Rule: RuleUnclear}
	}
	return SeverityResult{Severity: domain.Sev3, Rule: RuleUnmatched, NeedsReview: true}
}

// Route picks the ticket's team (HLD 4.3). A team confidence below
// TeamConfidenceMin, or a team ID the code does not know, goes to Service
// Desk with needs review; jev's preferred team stays on the Judgment.
func Route(j domain.Judgment) (domain.Team, bool) {
	team := domain.Team(j.Team.Option)
	if j.Team.Confidence < TeamConfidenceMin || !knownTeam(team) {
		return domain.TeamServiceDesk, true
	}
	return team, false
}

func knownTeam(t domain.Team) bool {
	for _, k := range domain.Teams {
		if k == t {
			return true
		}
	}
	return false
}
