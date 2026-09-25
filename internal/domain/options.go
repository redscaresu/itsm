// Package domain holds the ticket model, the jev option IDs and the
// interfaces the other packages build against (HLD sections 4.2, 6 and 7).
package domain

import "strconv"

// Team is a support team. Its value is the jev option ID for the "team"
// question (HLD 4.2). The client (S2) and the rules (S3) both use these
// constants, so the two cannot drift.
type Team string

// Team option IDs.
const (
	TeamNetwork        Team = "network"
	TeamEndUserCompute Team = "end_user_compute"
	TeamApplications   Team = "applications"
	TeamIdentityAccess Team = "identity_access"
	TeamServiceDesk    Team = "service_desk"
)

// Teams lists every team in display order, for filters and reassign menus.
var Teams = []Team{TeamNetwork, TeamEndUserCompute, TeamApplications, TeamIdentityAccess, TeamServiceDesk}

// Label is the team's display name. An unknown team shows its raw ID.
func (t Team) Label() string {
	switch t {
	case TeamNetwork:
		return "Network"
	case TeamEndUserCompute:
		return "End User Compute"
	case TeamApplications:
		return "Applications"
	case TeamIdentityAccess:
		return "Identity and Access"
	case TeamServiceDesk:
		return "Service Desk"
	}
	return string(t)
}

// Kind is the jev option ID for the "kind" question (HLD 4.2).
type Kind string

// Kind option IDs.
const (
	KindFault    Kind = "fault"
	KindChange   Kind = "change"
	KindQuestion Kind = "question"
)

// Scope is the jev option ID for the "scope" question (HLD 4.2).
type Scope string

// Scope option IDs.
const (
	ScopeOneUser   Scope = "one_user"
	ScopeOneOffice Scope = "one_office"
	ScopeRegion    Scope = "region"
	ScopeNational  Scope = "national"
	ScopeUnclear   Scope = "unclear"
)

// Label is the scope in words, as the decision feed shows it
// ("one office"). An unknown scope shows its raw ID.
func (s Scope) Label() string {
	switch s {
	case ScopeOneUser:
		return "one user"
	case ScopeOneOffice:
		return "one office"
	case ScopeRegion:
		return "region"
	case ScopeNational:
		return "national"
	case ScopeUnclear:
		return "unclear"
	}
	return string(s)
}

// Severity is 1 (worst) to 4. The zero value means "not triaged yet".
type Severity int

// Severity levels (HLD section 5).
const (
	Sev1 Severity = 1
	Sev2 Severity = 2
	Sev3 Severity = 3
	Sev4 Severity = 4
)

// Severities lists every severity in display order.
var Severities = []Severity{Sev1, Sev2, Sev3, Sev4}

// String renders "Sev 2"; the zero value renders as an empty string.
func (s Severity) String() string {
	if s == 0 {
		return ""
	}
	return "Sev " + strconv.Itoa(int(s))
}

// Status is where a ticket is in its lifecycle (HLD section 6).
type Status string

// Ticket statuses.
const (
	StatusTriaging  Status = "triaging"
	StatusNeedsInfo Status = "needs_info"
	StatusOpen      Status = "open"
	StatusClosed    Status = "closed"
)

// FollowUpID names a pre-written follow-up question (HLD 4.4). The customer
// form (S4) uses it as the answer field name; the rules (S3) write it.
type FollowUpID string

// Follow-up question IDs, in the row order of HLD 4.4.
const (
	FollowUpLocation FollowUpID = "location"
	FollowUpService  FollowUpID = "service"
	FollowUpImpact   FollowUpID = "impact"
)

// OverrideField names what an agent action changed (HLD 3.2 item 5).
type OverrideField string

// Override fields: reassign, change severity, close.
const (
	OverrideTeam     OverrideField = "team"
	OverrideSeverity OverrideField = "severity"
	OverrideStatus   OverrideField = "status"
)
