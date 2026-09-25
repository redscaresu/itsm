package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/store/memory"
	"github.com/redscaresu/itsm/internal/web"
)

// stubRunner records every Start and runs nothing.
type stubRunner struct {
	mu     sync.Mutex
	starts []string
}

func (r *stubRunner) Start(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, id)
}

func (r *stubRunner) Wait(context.Context) error { return nil }

func (r *stubRunner) started() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.starts...)
}

type customerFixture struct {
	h      http.Handler
	store  *memory.Store
	runner *stubRunner
}

func newCustomerFixture() customerFixture {
	st, rn := memory.New(), &stubRunner{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return customerFixture{h: NewServer(logger, Config{}, st, rn), store: st, runner: rn}
}

func (f customerFixture) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

// seed saves a ticket and then sets its status and follow-ups, as the
// runner would.
func (f customerFixture) seed(t *testing.T, status domain.Status, followUps ...domain.FollowUp) domain.Ticket {
	t.Helper()
	tk := domain.Ticket{Title: "Everything is broken", Description: "Everything is broken, please help."}
	require.NoError(t, f.store.Create(context.Background(), &tk))
	require.NoError(t, f.store.Update(context.Background(), tk.ID, func(t *domain.Ticket) error {
		t.Status, t.FollowUps = status, followUps
		t.Team, t.Severity = domain.TeamServiceDesk, domain.Sev3
		return nil
	}))
	got, err := f.store.Get(context.Background(), tk.ID)
	require.NoError(t, err)
	return got
}

func testIDAttr(id string) string { return `data-testid="` + id + `"` }

var ticket4FollowUps = []domain.FollowUp{
	{ID: domain.FollowUpLocation, Question: "Which office or site is affected? If it's everywhere, or just you, say so."},
	{ID: domain.FollowUpService, Question: "Which system, application or device has the problem?"},
	{ID: domain.FollowUpImpact, Question: "Roughly how many people are affected — just you, your office, or more?"},
}

var ticket4Answers = url.Values{"location": {"Bristol office"}, "service": {"OASys"}, "impact": {"just me"}}

// Regression: GET / stops serving the form, or "/" becomes a catch-all that
// answers 200 for every unknown path.
func TestCustomerHome(t *testing.T) {
	f := newCustomerFixture()
	rec := f.do(http.MethodGet, "/", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), testIDAttr(web.TestIDCustomerForm))
	assert.Equal(t, http.StatusNotFound, f.do(http.MethodGet, "/nope", nil).Code)
}

// Regression: the create handler stops redirecting, forgets to start the
// background run (the ticket sits triaging forever), starts it twice, or
// saves the ticket in some status other than triaging (HLD 3.1 item 2).
func TestCustomerCreateTicket(t *testing.T) {
	f := newCustomerFixture()
	rec := f.do(http.MethodPost, "/tickets", url.Values{
		"name": {"Sam Jones"}, "email": {"sam@example.com"},
		"title": {"Leeds office network down"}, "description": {"Nobody in Leeds can get on the network."},
	})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	loc := rec.Header().Get("Location")

	all, err := f.store.List(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 1)
	got := all[0]
	assert.Equal(t, "/tickets/"+got.ID, loc)
	assert.Equal(t, domain.StatusTriaging, got.Status)
	assert.Equal(t, domain.Customer{Name: "Sam Jones", Email: "sam@example.com"}, got.Customer)
	assert.Equal(t, "Leeds office network down", got.Title)
	assert.Equal(t, "Nobody in Leeds can get on the network.", got.Description)
	assert.Equal(t, []string{got.ID}, f.runner.started())

	page := f.do(http.MethodGet, loc, nil)
	assert.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), testIDAttr(web.TestIDCustomerTriaging))
}

// Regression: an invalid form is saved anyway, starts a run, or comes back
// without the GOV.UK error summary and a 400.
func TestCustomerCreateInvalid(t *testing.T) {
	cases := map[string]struct {
		form url.Values
		want []string
	}{
		"all empty": {
			form: url.Values{},
			want: []string{"Enter your name", "Enter your email address", "Enter a title", "Describe the problem"},
		},
		"bad email": {
			form: url.Values{"name": {"Sam"}, "email": {"sam at example"}, "title": {"x"}, "description": {"y"}},
			want: []string{"Enter an email address in the correct format"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCustomerFixture()
			rec := f.do(http.MethodPost, "/tickets", tc.form)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			body := rec.Body.String()
			assert.Contains(t, body, "govuk-error-summary")
			assert.Contains(t, body, testIDAttr(web.TestIDCustomerErrors))
			for _, msg := range tc.want {
				assert.Contains(t, body, msg)
			}
			all, err := f.store.List(context.Background())
			require.NoError(t, err)
			assert.Empty(t, all)
			assert.Empty(t, f.runner.started())
		})
	}
}

// Regression: the status partial 500s in some status, keeps polling after
// triage lands, stops polling while still triaging, or shows the follow-up
// form when no answers are wanted (HLD 3.1 item 3).
func TestTicketStatusPartial(t *testing.T) {
	cases := []struct {
		status       domain.Status
		wantPoll     bool
		wantFollowUp bool
	}{
		{domain.StatusTriaging, true, false},
		{domain.StatusNeedsInfo, false, true},
		{domain.StatusOpen, false, false},
		{domain.StatusClosed, false, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			f := newCustomerFixture()
			tk := f.seed(t, tc.status, ticket4FollowUps...)
			rec := f.do(http.MethodGet, "/tickets/"+tk.ID+"/status", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			body := rec.Body.String()
			assert.NotContains(t, body, "<html", "a fragment, not a page")
			assert.Contains(t, body, tk.ID)
			assert.Equal(t, tc.wantPoll, strings.Contains(body, `hx-trigger="every 1s"`), "polling")
			assert.Equal(t, tc.wantFollowUp, strings.Contains(body, testIDAttr(web.TestIDCustomerFollowUpForm)), "follow-up form")
			if tc.wantFollowUp {
				assert.Contains(t, body, testIDAttr(web.FollowUpAnswerTestID(domain.FollowUpLocation)))
				assert.Contains(t, body, "Which system, application or device has the problem?")
			}
		})
	}
}

// Regression: the partial ignores since, so the panel never flashes when
// triage lands, or flashes on every poll; a bad since or unknown ticket 500s.
func TestTicketStatusSinceAndErrors(t *testing.T) {
	f := newCustomerFixture()
	tk := f.seed(t, domain.StatusOpen)
	before := sinceValue(tk.UpdatedAt.Add(-time.Millisecond))
	at := sinceValue(tk.UpdatedAt)

	flashed := f.do(http.MethodGet, "/tickets/"+tk.ID+"/status?since="+before, nil)
	assert.Contains(t, flashed.Body.String(), `class="`+web.FlashClass+`"`)
	quiet := f.do(http.MethodGet, "/tickets/"+tk.ID+"/status?since="+at, nil)
	assert.NotContains(t, quiet.Body.String(), web.FlashClass)

	assert.Equal(t, http.StatusBadRequest, f.do(http.MethodGet, "/tickets/"+tk.ID+"/status?since=soon", nil).Code)
	assert.Equal(t, http.StatusNotFound, f.do(http.MethodGet, "/tickets/T-9999/status", nil).Code)
	assert.Equal(t, http.StatusNotFound, f.do(http.MethodGet, "/tickets/T-9999", nil).Code)
}

// Regression: follow-up answers are dropped or stored against the wrong
// question, the ticket is not set back to triaging, or the second run is
// not started, or started twice (HLD 3.1 item 4).
func TestFollowUpRecordsAnswers(t *testing.T) {
	f := newCustomerFixture()
	tk := f.seed(t, domain.StatusNeedsInfo, ticket4FollowUps...)

	rec := f.do(http.MethodPost, "/tickets/"+tk.ID+"/follow-up", ticket4Answers)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/tickets/"+tk.ID, rec.Header().Get("Location"))

	got, err := f.store.Get(context.Background(), tk.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusTriaging, got.Status)
	require.Len(t, got.FollowUps, len(ticket4FollowUps))
	for _, fu := range got.FollowUps {
		assert.Equal(t, ticket4Answers.Get(string(fu.ID)), fu.Answer, fu.ID)
		assert.False(t, fu.AnsweredAt.IsZero(), fu.ID)
	}
	assert.Equal(t, []string{tk.ID}, f.runner.started())
}

// Regression: a second follow-up round is accepted, so the ticket can loop
// between needs_info and triaging (HLD 3.1 item 4: one round only). The
// second case holds even if a faulty run set needs_info again.
func TestFollowUpSecondRoundConflict(t *testing.T) {
	f := newCustomerFixture()
	tk := f.seed(t, domain.StatusNeedsInfo, ticket4FollowUps...)
	require.Equal(t, http.StatusSeeOther, f.do(http.MethodPost, "/tickets/"+tk.ID+"/follow-up", ticket4Answers).Code)

	assert.Equal(t, http.StatusConflict, f.do(http.MethodPost, "/tickets/"+tk.ID+"/follow-up", ticket4Answers).Code, "while triaging")

	require.NoError(t, f.store.Update(context.Background(), tk.ID, func(t *domain.Ticket) error {
		t.Status = domain.StatusNeedsInfo
		return nil
	}))
	assert.Equal(t, http.StatusConflict, f.do(http.MethodPost, "/tickets/"+tk.ID+"/follow-up", ticket4Answers).Code, "answers already given")

	open := f.seed(t, domain.StatusOpen)
	assert.Equal(t, http.StatusConflict, f.do(http.MethodPost, "/tickets/"+open.ID+"/follow-up", ticket4Answers).Code, "open ticket")
	assert.Equal(t, []string{tk.ID}, f.runner.started(), "only the first round starts a run")
}

// Regression: a blank answer is saved and a run started, or the form comes
// back without the error summary and the customer's other answers.
func TestFollowUpMissingAnswer(t *testing.T) {
	f := newCustomerFixture()
	tk := f.seed(t, domain.StatusNeedsInfo, ticket4FollowUps...)

	rec := f.do(http.MethodPost, "/tickets/"+tk.ID+"/follow-up", url.Values{"location": {"Bristol office"}, "service": {" "}})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, testIDAttr(web.TestIDCustomerErrors))
	assert.Contains(t, body, `href="#follow-up-service"`)
	assert.Contains(t, body, `href="#follow-up-impact"`)
	assert.Contains(t, body, ">Bristol office</textarea>", "keeps what was typed")

	got, err := f.store.Get(context.Background(), tk.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusNeedsInfo, got.Status)
	assert.Equal(t, tk.FollowUps, got.FollowUps)
	assert.Empty(t, f.runner.started())
}
