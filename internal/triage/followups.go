package triage

import "github.com/redscaresu/itsm/internal/domain"

// Pre-written follow-up questions (HLD 4.4). jev never writes these.
const (
	QuestionLocation = "Which office or site is affected? If it's everywhere, or just you, say so."
	QuestionService  = "Which system, application or device has the problem?"
	QuestionImpact   = "Roughly how many people are affected — just you, your office, or more?"
)

// FollowUpTriggered reports whether triage could not decide (HLD 4.4):
// scope is unclear, or team confidence is below TeamConfidenceMin. A ticket
// jev placed confidently is never interrupted.
func FollowUpTriggered(j domain.Judgment) bool {
	return domain.Scope(j.Scope.Option) == domain.ScopeUnclear || j.Team.Confidence < TeamConfidenceMin
}

// SelectFollowUps returns the questions for what is missing, in the HLD 4.4
// row order, with empty answers. It does not check the trigger.
func SelectFollowUps(j domain.Judgment) []domain.FollowUp {
	var out []domain.FollowUp
	if j.StatesLocation < DetailMissingMax {
		out = append(out, domain.FollowUp{ID: domain.FollowUpLocation, Question: QuestionLocation})
	}
	if j.StatesService < DetailMissingMax {
		out = append(out, domain.FollowUp{ID: domain.FollowUpService, Question: QuestionService})
	}
	if domain.Scope(j.Scope.Option) == domain.ScopeUnclear {
		out = append(out, domain.FollowUp{ID: domain.FollowUpImpact, Question: QuestionImpact})
	}
	return out
}

// Outcome is everything one successful run decides for the ticket.
type Outcome struct {
	Team        domain.Team
	Severity    domain.Severity
	Rule        string
	NeedsReview bool
	Status      domain.Status
	// FollowUps is non-empty only when the ticket moves to needs_info.
	FollowUps []domain.FollowUp
}

// Decide applies routing (4.3), the severity rules (5) and follow-up
// selection (4.4) to one run. firstRound is true when the ticket has had no
// follow-up round yet; only then can the outcome be needs_info, so there is
// at most one round.
//
// Needs review is set when routing, rule 7 or the follow-up trigger says so:
// a ticket triage could not decide is flagged for an agent whether or not
// the customer is asked.
func Decide(j domain.Judgment, firstRound bool) Outcome {
	team, routeReview := Route(j)
	sev := Severity(j)
	triggered := FollowUpTriggered(j)
	o := Outcome{
		Team:        team,
		Severity:    sev.Severity,
		Rule:        sev.Rule,
		NeedsReview: routeReview || sev.NeedsReview || triggered,
		Status:      domain.StatusOpen,
	}
	if !triggered || !firstRound {
		return o
	}
	if fs := SelectFollowUps(j); len(fs) > 0 {
		o.Status = domain.StatusNeedsInfo
		o.FollowUps = fs
	}
	return o
}
