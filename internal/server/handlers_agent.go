package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/web"
)

// Query parameters and form fields for the agent screen.
const (
	teamParam     = "team"
	severityParam = "severity"
)

// queueFilter is the team / severity filter from the query string (HLD 3.2).
// A zero field means "all".
type queueFilter struct {
	Team     domain.Team
	Severity domain.Severity
}

// parseQueueFilter reads the filter. An unknown team or severity is an
// error (answer 400) rather than a silently empty queue.
func parseQueueFilter(r *http.Request) (queueFilter, error) {
	q := r.URL.Query()
	var f queueFilter
	if raw := q.Get(teamParam); raw != "" {
		f.Team = domain.Team(raw)
		if !slices.Contains(domain.Teams, f.Team) {
			return f, fmt.Errorf("unknown team %q", raw)
		}
	}
	if raw := q.Get(severityParam); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || !slices.Contains(domain.Severities, domain.Severity(n)) {
			return f, fmt.Errorf("unknown severity %q", raw)
		}
		f.Severity = domain.Severity(n)
	}
	return f, nil
}

func (f queueFilter) keep(t domain.Ticket) bool {
	return (f.Team == "" || t.Team == f.Team) && (f.Severity == 0 || t.Severity == f.Severity)
}

// pollURL is the URL of the next poll: path, the filter, and since.
func (f queueFilter) pollURL(path string, next time.Time) string {
	q := url.Values{sinceParam: {sinceValue(next)}}
	if f.Team != "" {
		q.Set(teamParam, string(f.Team))
	}
	if f.Severity != 0 {
		q.Set(severityParam, strconv.Itoa(int(f.Severity)))
	}
	return path + "?" + q.Encode()
}

// queueRow is one ticket in the queue table.
type queueRow struct {
	domain.Ticket
	Class    string // flash and / or triaging classes
	Triaging bool
	Error    string // the latest run's jev error, shown in the row (HLD 4.6)
}

// queueView is the data for agent/_queue.html.
type queueView struct {
	Filter     queueFilter
	Teams      []domain.Team
	Severities []domain.Severity
	Rows       []queueRow
	PollURL    string
}

// buildQueue filters tickets and marks rows updated after since. next is
// the since value for the following poll. Callers read the clock before
// listing, so a change that races the list flashes twice rather than never.
func buildQueue(tickets []domain.Ticket, f queueFilter, since, next time.Time) queueView {
	v := queueView{Filter: f, Teams: domain.Teams, Severities: domain.Severities, PollURL: f.pollURL("/agent/queue", next)}
	for _, t := range tickets {
		if !f.keep(t) {
			continue
		}
		row := queueRow{Ticket: t, Triaging: t.Status == domain.StatusTriaging}
		if n := len(t.Runs); n > 0 {
			row.Error = t.Runs[n-1].Error
		}
		var classes []string
		if c := flashClass(t.UpdatedAt, since); c != "" {
			classes = append(classes, c)
		}
		if row.Triaging {
			classes = append(classes, web.TriagingClass)
		}
		row.Class = strings.Join(classes, " ")
		v.Rows = append(v.Rows, row)
	}
	return v
}

// feedLine is one completed triage run in the decision feed (HLD 3.2).
type feedLine struct {
	TicketID string
	Run      domain.Judgment
	Class    string
}

// TeamLabel is the run's team in words.
func (l feedLine) TeamLabel() string { return domain.Team(l.Run.Team.Option).Label() }

// ScopeLabel is the run's scope in words ("one office").
func (l feedLine) ScopeLabel() string { return domain.Scope(l.Run.Scope.Option).Label() }

// FailedTeamLabel is the team a failed run routes to (HLD 4.6). A failed
// run's own fields are zero, so the feed names the fixed failure team.
func (feedLine) FailedTeamLabel() string { return domain.TeamServiceDesk.Label() }

// feedView is the data for agent/_feed.html.
type feedView struct {
	Lines   []feedLine
	PollURL string
}

// buildFeed lists every run of every ticket, newest CompletedAt first.
// Lines completed after since flash, like queue rows.
func buildFeed(tickets []domain.Ticket, since, next time.Time) feedView {
	v := feedView{PollURL: "/agent/feed?" + url.Values{sinceParam: {sinceValue(next)}}.Encode()}
	for _, t := range tickets {
		for _, run := range t.Runs {
			v.Lines = append(v.Lines, feedLine{TicketID: t.ID, Run: run, Class: flashClass(run.CompletedAt, since)})
		}
	}
	sort.SliceStable(v.Lines, func(i, j int) bool {
		return v.Lines[i].Run.CompletedAt.After(v.Lines[j].Run.CompletedAt)
	})
	return v
}

// ruleWords says each section 5 rule string in words, for the Why panel.
// These keys must match the rule strings in internal/triage/rules.go (S3);
// an unknown rule shows as its raw string.
var ruleWords = map[string]string{
	"change-or-question":   "A change, question or cosmetic issue — not an incident",
	"national":             "All offices, or a national service, down",
	"one-office-or-region": "One office or region down",
	"one-user-blocked":     "One user blocked",
	"one-user-working":     "Degraded but working",
	"unclear":              "Scope unclear, so provisional",
	"unmatched":            "An answer the rules do not know; needs review",
}

// probRow is one labelled probability in the Why panel.
type probRow struct {
	Label  string
	P      float64
	Chosen bool
}

// Percent is P as a whole percentage, for bar widths.
func (p probRow) Percent() int { return int(p.P*100 + 0.5) }

// whyRun is one triage run as the Why panel shows it.
type whyRun struct {
	N         int
	Run       domain.Judgment
	TeamBars  []probRow
	Kind      []probRow
	Scope     []probRow
	Nouls     []probRow
	RuleWords string
}

func newWhyRun(n int, j domain.Judgment) whyRun {
	w := whyRun{N: n, Run: j, RuleWords: j.Rule}
	if words, ok := ruleWords[j.Rule]; ok {
		w.RuleWords = words
	}
	for _, team := range domain.Teams {
		w.TeamBars = append(w.TeamBars, probRow{
			Label: team.Label(), P: j.Team.Probabilities[string(team)], Chosen: string(team) == j.Team.Option,
		})
	}
	w.Kind = choiceRows(j.Kind, func(k string) string { return k })
	w.Scope = choiceRows(j.Scope, func(k string) string { return domain.Scope(k).Label() })
	w.Nouls = []probRow{
		{Label: "Someone cannot work (blocked)", P: j.Blocked},
		{Label: "Says where (states_location)", P: j.StatesLocation},
		{Label: "Says which system (states_service)", P: j.StatesService},
	}
	return w
}

// choiceRows lists a choice's probabilities, highest first.
func choiceRows(c domain.Choice, label func(string) string) []probRow {
	rows := make([]probRow, 0, len(c.Probabilities))
	for k, p := range c.Probabilities {
		rows = append(rows, probRow{Label: label(k), P: p, Chosen: k == c.Option})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].P != rows[j].P {
			return rows[i].P > rows[j].P
		}
		return rows[i].Label < rows[j].Label
	})
	return rows
}

// ticketView is the data for agent/ticket.html.
type ticketView struct {
	Ticket     domain.Ticket
	Runs       []whyRun
	Locked     bool // actions disabled: triaging or closed
	Teams      []domain.Team
	Severities []domain.Severity
}

func newTicketView(t domain.Ticket) ticketView {
	v := ticketView{Ticket: t, Locked: actionAllowed(t.Status) != nil, Teams: domain.Teams, Severities: domain.Severities}
	for i, j := range t.Runs {
		v.Runs = append(v.Runs, newWhyRun(i+1, j))
	}
	return v
}

// Errors an agent action answers 409 for (HLD 3.2 item 5, section 6).
var (
	errTriaging = errors.New("ticket is being triaged; try again when it lands")
	errClosed   = errors.New("ticket is closed")
)

// actionAllowed reports whether an agent may act on a ticket in status s.
func actionAllowed(s domain.Status) error {
	switch s {
	case domain.StatusTriaging:
		return errTriaging
	case domain.StatusClosed:
		return errClosed
	}
	return nil
}

// agentPage is the data for agent/queue.html: the table and the feed.
type agentPage struct {
	Queue queueView
	Feed  feedView
}

// handleAgentQueuePage serves GET /agent. Nothing flashes on a page load.
func handleAgentQueuePage(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := parseQueueFilter(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		now := d.clock()
		tickets, ok := d.listTickets(w, r)
		if !ok {
			return
		}
		d.render(w, http.StatusOK, "agent/queue.html", agentPage{
			Queue: buildQueue(tickets, f, now, now),
			Feed:  buildFeed(tickets, now, now),
		})
	})
}

// handleAgentQueue serves GET /agent/queue, the polling table (HLD 3.2).
func handleAgentQueue(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := parseQueueFilter(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		since, err := d.parseSince(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		next := d.clock()
		tickets, ok := d.listTickets(w, r)
		if !ok {
			return
		}
		d.render(w, http.StatusOK, "agent/_queue.html", buildQueue(tickets, f, since, next))
	})
}

// handleAgentFeed serves GET /agent/feed, the polling decision feed.
func handleAgentFeed(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, err := d.parseSince(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		next := d.clock()
		tickets, ok := d.listTickets(w, r)
		if !ok {
			return
		}
		d.render(w, http.StatusOK, "agent/_feed.html", buildFeed(tickets, since, next))
	})
}

func (d deps) listTickets(w http.ResponseWriter, r *http.Request) ([]domain.Ticket, bool) {
	tickets, err := d.store.List(r.Context())
	if err != nil {
		d.logger.Error("list tickets", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, false
	}
	return tickets, true
}

// handleAgentTicket serves GET /agent/tickets/{id}: the ticket and the Why panel.
func handleAgentTicket(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, err := d.store.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			d.storeError(w, err)
			return
		}
		d.render(w, http.StatusOK, "agent/ticket.html", newTicketView(t))
	})
}

// storeError answers 404 for a missing ticket, 409 for a refused action,
// and 500 for anything else.
func (d deps) storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "No such ticket", http.StatusNotFound)
	case errors.Is(err, errTriaging), errors.Is(err, errClosed):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		d.logger.Error("store", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// reassignForm is the body of POST /agent/tickets/{id}/reassign.
type reassignForm struct {
	Team string `form:"team"`
}

// Valid accepts only a known team.
func (f reassignForm) Valid(context.Context) map[string]string {
	if !slices.Contains(domain.Teams, domain.Team(f.Team)) {
		return map[string]string{teamParam: "Choose a team"}
	}
	return nil
}

// severityForm is the body of POST /agent/tickets/{id}/severity.
type severityForm struct {
	Severity int `form:"severity"`
}

// Valid accepts only Sev 1 to Sev 4.
func (f severityForm) Valid(context.Context) map[string]string {
	if !slices.Contains(domain.Severities, domain.Severity(f.Severity)) {
		return map[string]string{severityParam: "Choose a severity"}
	}
	return nil
}

// handleAgentReassign changes the team. A needs_info ticket moves to open.
func handleAgentReassign(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := decodeAction[reassignForm](w, r)
		if !ok {
			return
		}
		to := domain.Team(f.Team)
		d.act(w, r, func(t *domain.Ticket) domain.Override {
			o := domain.Override{Field: domain.OverrideTeam, From: t.Team.Label(), To: to.Label()}
			t.Team = to
			openIfNeedsInfo(t)
			return o
		})
	})
}

// handleAgentSeverity changes the severity. A needs_info ticket moves to open.
func handleAgentSeverity(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := decodeAction[severityForm](w, r)
		if !ok {
			return
		}
		to := domain.Severity(f.Severity)
		d.act(w, r, func(t *domain.Ticket) domain.Override {
			o := domain.Override{Field: domain.OverrideSeverity, From: t.Severity.String(), To: to.String()}
			t.Severity = to
			openIfNeedsInfo(t)
			return o
		})
	})
}

// handleAgentClose closes a non-closed ticket.
func handleAgentClose(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.act(w, r, func(t *domain.Ticket) domain.Override {
			o := domain.Override{Field: domain.OverrideStatus, From: string(t.Status), To: string(domain.StatusClosed)}
			t.Status = domain.StatusClosed
			return o
		})
	})
}

func openIfNeedsInfo(t *domain.Ticket) {
	if t.Status == domain.StatusNeedsInfo {
		t.Status = domain.StatusOpen
	}
}

// decodeAction decodes an action form; a bad one answers 400. The forms
// only offer valid values, so there is no form to re-render.
func decodeAction[T Validator](w http.ResponseWriter, r *http.Request) (T, bool) {
	f, problems, err := decodeForm[T](r)
	if err != nil || len(problems) > 0 {
		http.Error(w, "Bad request: choose a value from the list", http.StatusBadRequest)
		return f, false
	}
	return f, true
}

// act applies one agent action under the store's lock and appends its
// Override. The status check runs inside Update, so a triage run that
// starts between page load and POST still gets a 409, and a background
// run can never overwrite the agent's change (HLD 3.2 item 5).
func (d deps) act(w http.ResponseWriter, r *http.Request, change func(*domain.Ticket) domain.Override) {
	id := r.PathValue("id")
	err := d.store.Update(r.Context(), id, func(t *domain.Ticket) error {
		if err := actionAllowed(t.Status); err != nil {
			return err
		}
		o := change(t)
		o.At = d.clock()
		t.Overrides = append(t.Overrides, o)
		return nil
	})
	if err != nil {
		d.storeError(w, err)
		return
	}
	http.Redirect(w, r, "/agent/tickets/"+url.PathEscape(id), http.StatusSeeOther)
}
