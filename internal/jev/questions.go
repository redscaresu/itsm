package jev

import "github.com/redscaresu/itsm/internal/domain"

// Question IDs (HLD 4.2). They are for code only; jev sees the instructions
// and criteria.
const (
	QuestionTeam           = "team"
	QuestionKind           = "kind"
	QuestionScope          = "scope"
	QuestionBlocked        = "blocked"
	QuestionStatesLocation = "states_location"
	QuestionStatesService  = "states_service"
)

// Answer types jev uses (HLD 4.1).
const (
	TypeChoice = "choice"
	TypeNoul   = "noul"
)

// Question is one entry in the request's "questions" object.
type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

// questions returns the HLD 4.2 question set. The wording is the contract:
// copy changes from the HLD, never edit here alone. Option IDs come from
// domain so the rules (S3) and this client cannot drift.
func questions() map[string]Question {
	return map[string]Question{
		QuestionTeam: {
			Type:         TypeChoice,
			Instructions: "Which support team should handle `ticket`? Use `context.teams` for what each team covers. If the ticket is too vague to place, or fits none of the teams, choose service_desk.",
			Criteria: map[string]string{
				string(domain.TeamNetwork):        "connectivity, VPN, Wi-Fi, a whole site losing connection, or a change to network kit such as firewall rules, ports, switches or cabling",
				string(domain.TeamEndUserCompute): "a laptop, desktop, printer, phone or other device a person uses directly",
				string(domain.TeamApplications):   "a business application or case system such as nDelius or OASys, or a shared drive",
				string(domain.TeamIdentityAccess): "passwords, account lockouts, MFA, permissions or access to a system",
				string(domain.TeamServiceDesk):    "none of the above fits, or the ticket is too vague to tell",
			},
		},
		QuestionKind: {
			Type:         TypeChoice,
			Instructions: "What kind of ticket is `ticket`?",
			Criteria: map[string]string{
				string(domain.KindFault):    "something that used to work is broken, down or degraded",
				string(domain.KindChange):   "a request to add, change or remove something in a setup that is working: a port, a rule, a permission, a new device or account",
				string(domain.KindQuestion): "a how-to question, a request for advice, or a cosmetic issue that stops no one",
			},
		},
		QuestionScope: {
			Type:         TypeChoice,
			Instructions: "How widely is the problem in `ticket` felt? Use `ticket.follow_up_answers` if present.",
			Criteria: map[string]string{
				string(domain.ScopeOneUser):   "only the reporter, or one named person, is affected",
				string(domain.ScopeOneOffice): "a single probation office, site or building is affected",
				string(domain.ScopeRegion):    "several offices in one area or region are affected",
				string(domain.ScopeNational):  "every office is affected, or everyone who uses a national service is; not one person's or one office's problem with a national system",
				string(domain.ScopeUnclear):   "the ticket does not say, and it cannot be told, how widely the problem is felt",
			},
		},
		QuestionBlocked: {
			Type:         TypeNoul,
			Instructions: "Is at least one person unable to do their work because of the problem in `ticket`?",
			Criteria: map[string]string{
				"true":  "someone cannot do a task at all until this is fixed",
				"false": "work continues, perhaps more slowly or with a workaround, or nothing is broken",
			},
		},
		QuestionStatesLocation: {
			Type:         TypeNoul,
			Instructions: "Does `ticket` say which office, site or region is affected, or make clear that the problem is national or only affects the reporter? Count `ticket.follow_up_answers` as part of the ticket.",
		},
		QuestionStatesService: {
			Type:         TypeNoul,
			Instructions: "Does `ticket` name the system, application, device or service that has the problem? Count `ticket.follow_up_answers` as part of the ticket.",
		},
	}
}

// Organisation and team descriptions sent as state.context (HLD 4.1).
const organisation = "Probation Service, England and Wales. Staff work from probation offices; some services are national."

func teamContext() map[string]string {
	return map[string]string{
		string(domain.TeamNetwork):        "connectivity, VPN, Wi-Fi, whole-site outages, and changes to network kit: firewall rules, ports, switches, cabling",
		string(domain.TeamEndUserCompute): "laptops, desktops, printers, phones, peripherals a person uses directly",
		string(domain.TeamApplications):   "case-management and business applications such as nDelius, OASys, and shared drives",
		string(domain.TeamIdentityAccess): "passwords, accounts, MFA, permissions, access to systems",
		string(domain.TeamServiceDesk):    "anything that fits none of the above, or is too vague to place",
	}
}

// State is the request's "state" object (HLD 4.1).
type State struct {
	Ticket  TicketState  `json:"ticket"`
	Context StateContext `json:"context"`
}

// TicketState is state.ticket. FollowUpAnswers is never nil, so the first
// call sends [] rather than null.
type TicketState struct {
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	FollowUpAnswers []FollowUpAnswer `json:"follow_up_answers"`
}

// FollowUpAnswer is one answered follow-up question.
type FollowUpAnswer struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// StateContext is state.context.
type StateContext struct {
	Organisation string            `json:"organisation"`
	Teams        map[string]string `json:"teams"`
}

// NewState builds the state for one ticket. answers may be nil.
func NewState(title, description string, answers []FollowUpAnswer) State {
	if answers == nil {
		answers = []FollowUpAnswer{}
	}
	return State{
		Ticket: TicketState{Title: title, Description: description, FollowUpAnswers: answers},
		Context: StateContext{
			Organisation: organisation,
			Teams:        teamContext(),
		},
	}
}
