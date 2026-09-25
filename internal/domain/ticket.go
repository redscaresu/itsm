package domain

import "time"

// Customer is who raised the ticket.
type Customer struct {
	Name  string
	Email string
}

// FollowUp is one pre-written question and the customer's answer.
// The runner writes it with an empty Answer (HLD 4.4); the customer fills it.
type FollowUp struct {
	ID         FollowUpID
	Question   string
	Answer     string
	AnsweredAt time.Time
}

// Override is one agent action. From and To are display strings.
type Override struct {
	Field OverrideField
	From  string
	To    string
	At    time.Time
}

// Choice is jev's answer to a "choice" question. Option holds the option ID;
// compare it with the typed constants, e.g. Team(j.Team.Option) == TeamNetwork.
type Choice struct {
	Option        string
	Confidence    float64
	Probabilities map[string]float64
}

// Usage is jev's token count for one call.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Judgment is everything about one triage run, so the feed and the Why panel
// read per run and never from the ticket's current values (HLD section 6).
type Judgment struct {
	CompletedAt time.Time
	Model       string
	Latency     time.Duration
	Usage       Usage

	Team  Choice
	Kind  Choice
	Scope Choice

	Blocked        float64
	StatesLocation float64
	StatesService  float64

	Severity    Severity
	Rule        string
	NeedsReview bool

	// Error is empty unless the call failed; then the jev fields are zero.
	Error string
}

// Ticket is one customer ticket (HLD section 6).
type Ticket struct {
	ID          string
	CreatedAt   time.Time
	Customer    Customer
	Title       string
	Description string
	FollowUps   []FollowUp

	Team        Team     // zero value while triaging
	Severity    Severity // zero value while triaging
	Status      Status
	NeedsReview bool

	Runs      []Judgment // one per triage run, at most two
	Overrides []Override

	// UpdatedAt is bumped by the store on every change. The polling
	// partials use it to decide which rows flash.
	UpdatedAt time.Time
}

// Clone returns a deep copy, so a caller holding it cannot change the
// original through a shared slice or map.
func (t Ticket) Clone() Ticket {
	c := t
	c.FollowUps = append([]FollowUp(nil), t.FollowUps...)
	c.Overrides = append([]Override(nil), t.Overrides...)
	c.Runs = nil
	for _, j := range t.Runs {
		j.Team = j.Team.clone()
		j.Kind = j.Kind.clone()
		j.Scope = j.Scope.clone()
		c.Runs = append(c.Runs, j)
	}
	return c
}

func (c Choice) clone() Choice {
	if c.Probabilities == nil {
		return c
	}
	p := make(map[string]float64, len(c.Probabilities))
	for k, v := range c.Probabilities {
		p[k] = v
	}
	c.Probabilities = p
	return c
}
