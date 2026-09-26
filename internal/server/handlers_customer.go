package server

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
)

// Customer form field names. The template uses the same names.
const (
	fieldName        = "name"
	fieldEmail       = "email"
	fieldTitle       = "title"
	fieldDescription = "description"
)

// Page and fragment templates for the customer screen.
const (
	tmplCustomerNew    = "customer/new.html"
	tmplCustomerTicket = "customer/ticket.html"
	tmplCustomerStatus = "customer/_status.html"
)

// formError is one entry in the GOV.UK error summary. Field is the id of
// the input it links to.
type formError struct {
	Field   string
	Message string
}

// newTicketForm is the customer form on GET / (HLD 3.1).
type newTicketForm struct {
	Name        string `form:"name"`
	Email       string `form:"email"`
	Title       string `form:"title"`
	Description string `form:"description"`
}

// Valid checks every field is filled in and the email looks like one.
func (f newTicketForm) Valid(context.Context) map[string]string {
	problems := map[string]string{}
	if strings.TrimSpace(f.Name) == "" {
		problems[fieldName] = "Enter your name"
	}
	switch email := strings.TrimSpace(f.Email); {
	case email == "":
		problems[fieldEmail] = "Enter your email address"
	case !isEmail(email):
		problems[fieldEmail] = "Enter an email address in the correct format, like name@example.com"
	}
	if strings.TrimSpace(f.Title) == "" {
		problems[fieldTitle] = "Enter a title"
	}
	if strings.TrimSpace(f.Description) == "" {
		problems[fieldDescription] = "Describe the problem"
	}
	return problems
}

func isEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s
}

// newTicketPage is the data for customer/new.html.
type newTicketPage struct {
	Form     newTicketForm
	Problems map[string]string
	Errors   []formError // Problems in form order, for the error summary
}

func newTicketPageFor(f newTicketForm, problems map[string]string) newTicketPage {
	return newTicketPage{
		Form:     f,
		Problems: problems,
		Errors:   orderedErrors(problems, fieldName, fieldEmail, fieldTitle, fieldDescription),
	}
}

// orderedErrors lists problems in the order the fields appear on the page.
func orderedErrors(problems map[string]string, fields ...string) []formError {
	var out []formError
	for _, f := range fields {
		if msg, ok := problems[f]; ok {
			out = append(out, formError{Field: f, Message: msg})
		}
	}
	return out
}

// handleCustomerHome shows the empty form.
func handleCustomerHome(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		d.render(w, http.StatusOK, tmplCustomerNew, newTicketPageFor(newTicketForm{}, nil))
	})
}

// handleTicketCreate saves the ticket as triaging, starts the background
// run and redirects to the ticket page. It never waits for jev (HLD 3.1).
func handleTicketCreate(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, problems, err := decodeForm[newTicketForm](r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if len(problems) > 0 {
			d.render(w, http.StatusBadRequest, tmplCustomerNew, newTicketPageFor(f, problems))
			return
		}
		t := domain.Ticket{
			Customer:    domain.Customer{Name: strings.TrimSpace(f.Name), Email: strings.TrimSpace(f.Email)},
			Title:       strings.TrimSpace(f.Title),
			Description: strings.TrimSpace(f.Description),
			Status:      domain.StatusTriaging,
		}
		if err := d.store.Create(r.Context(), &t); err != nil {
			d.serverError(w, "create ticket", err)
			return
		}
		d.runner.Start(t.ID)
		http.Redirect(w, r, ticketPath(t.ID), http.StatusSeeOther)
	})
}

func ticketPath(id string) string { return "/tickets/" + id }

// serverError logs err and answers 500 without leaking it.
func (d deps) serverError(w http.ResponseWriter, what string, err error) {
	d.logger.Error(what, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// statusLabels are the customer-facing words for each status.
var statusLabels = map[domain.Status]string{
	domain.StatusTriaging:  "Triaging",
	domain.StatusNeedsInfo: "More information needed",
	domain.StatusOpen:      "Open",
	domain.StatusClosed:    "Closed",
}

// customerTicketView is the data for customer/ticket.html and customer/_status.html.
type customerTicketView struct {
	Ticket      domain.Ticket
	StatusLabel string
	Triaging    bool   // show the "working out" text and keep polling
	NeedsInfo   bool   // show the follow-up form
	PollURL     string // next status poll, carrying since = this render's UpdatedAt
	Flash       string // web.FlashClass when the ticket changed since the last poll
	FollowUps   []followUpView
	Errors      []formError
}

// followUpView is one question on the follow-up form.
type followUpView struct {
	ID       domain.FollowUpID // the form field name
	Field    string            // the HTML id; the error summary links to it
	Question string
	Answer   string // what the customer typed, when re-rendering with errors
	Problem  string
}

func newCustomerTicketView(t domain.Ticket, flash string) customerTicketView {
	v := customerTicketView{
		Ticket:      t,
		StatusLabel: statusLabels[t.Status],
		Triaging:    t.Status == domain.StatusTriaging,
		NeedsInfo:   t.Status == domain.StatusNeedsInfo,
		PollURL:     ticketPath(t.ID) + "/status?" + sinceParam + "=" + sinceValue(t.UpdatedAt),
		Flash:       flash,
	}
	for _, f := range t.FollowUps {
		v.FollowUps = append(v.FollowUps, followUpView{
			ID: f.ID, Field: followUpField(f.ID), Question: f.Question, Answer: f.Answer,
		})
	}
	return v
}

// followUpField is the HTML id of a follow-up answer box; the form field
// name is the FollowUpID itself (domain.FollowUpID).
func followUpField(id domain.FollowUpID) string { return "follow-up-" + string(id) }

// loadTicket gets the {id} ticket, answering 404 or 500 itself on failure.
func (d deps) loadTicket(w http.ResponseWriter, r *http.Request) (domain.Ticket, bool) {
	t, err := d.store.Get(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.NotFound(w, r)
		return t, false
	case err != nil:
		d.serverError(w, "get ticket", err)
		return t, false
	}
	return t, true
}

// handleTicketPage shows one ticket; the status panel inside it polls.
func handleTicketPage(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, ok := d.loadTicket(w, r)
		if !ok {
			return
		}
		d.render(w, http.StatusOK, tmplCustomerTicket, newCustomerTicketView(t, ""))
	})
}

// handleTicketStatus is the polling partial (HLD 3.1, 7). The panel
// flashes when the ticket changed after since.
func handleTicketStatus(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, err := d.parseSince(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		t, ok := d.loadTicket(w, r)
		if !ok {
			return
		}
		d.render(w, http.StatusOK, tmplCustomerStatus, newCustomerTicketView(t, flashClass(t.UpdatedAt, since)))
	})
}

var (
	// errNoFollowUpRound: the ticket is not waiting for answers, or has
	// already had its one round (HLD 3.1 item 4).
	errNoFollowUpRound = errors.New("ticket is not waiting for follow-up answers")
	errAnswersMissing  = errors.New("follow-up answers missing")
)

// handleFollowUp records the answers, sets the ticket back to triaging and
// starts the second run. One round only: anything but a needs_info ticket
// whose questions are all unanswered gets 409.
func handleFollowUp(d deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		id := r.PathValue("id")
		var view customerTicketView
		err := d.store.Update(r.Context(), id, func(t *domain.Ticket) error {
			return recordAnswers(t, r.PostForm.Get, d.clock(), &view)
		})
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.NotFound(w, r)
		case errors.Is(err, errNoFollowUpRound):
			http.Error(w, "This ticket is not waiting for answers.", http.StatusConflict)
		case errors.Is(err, errAnswersMissing):
			d.render(w, http.StatusBadRequest, tmplCustomerTicket, view)
		case err != nil:
			d.serverError(w, "record follow-up answers", err)
		default:
			d.runner.Start(id)
			http.Redirect(w, r, ticketPath(id), http.StatusSeeOther)
		}
	})
}

// recordAnswers is the Update function for handleFollowUp. On missing
// answers it fills view so the handler can re-render the form with errors.
func recordAnswers(t *domain.Ticket, get func(string) string, now time.Time, view *customerTicketView) error {
	if t.Status != domain.StatusNeedsInfo || len(t.FollowUps) == 0 {
		return errNoFollowUpRound
	}
	*view = newCustomerTicketView(*t, "")
	for i := range view.FollowUps {
		if !t.FollowUps[i].AnsweredAt.IsZero() {
			return errNoFollowUpRound
		}
		f := &view.FollowUps[i]
		f.Answer = strings.TrimSpace(get(string(f.ID)))
		if f.Answer == "" {
			f.Problem = "Answer this question"
			view.Errors = append(view.Errors, formError{Field: f.Field, Message: f.Problem})
		}
	}
	if len(view.Errors) > 0 {
		return errAnswersMissing
	}
	for i, f := range view.FollowUps {
		t.FollowUps[i].Answer = f.Answer
		t.FollowUps[i].AnsweredAt = now
	}
	t.Status = domain.StatusTriaging
	return nil
}
