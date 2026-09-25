// Package triage turns jev's answers into decisions: severity (HLD section
// 5), routing and follow-up questions (HLD 4.3, 4.4), and the background
// runner that applies them to a ticket (HLD section 7).
package triage

// Thresholds (HLD 4.3). Starting values, not findings; S6 tunes them against
// live jev.
const (
	// TeamConfidenceMin: below this, route to Service Desk and set needs
	// review. Also a follow-up trigger (HLD 4.4).
	TeamConfidenceMin = 0.70
	// NoulYes: a noul at or above this counts as yes.
	NoulYes = 0.50
	// DetailMissingMax: states_location or states_service below this selects
	// the matching follow-up question (HLD 4.4).
	DetailMissingMax = 0.50
)
