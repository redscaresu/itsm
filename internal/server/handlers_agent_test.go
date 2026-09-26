package server

import (
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/store/memory"
	"github.com/redscaresu/itsm/internal/web"
)

// newAgentServer is the agent routes over a fresh memory store.
func newAgentServer(t *testing.T) (*memory.Store, http.Handler) {
	t.Helper()
	st := memory.New()
	d := deps{
		store:     st,
		runner:    domain.NopRunner{},
		templates: web.MustParse(),
		clock:     time.Now,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	addAgentRoutes(mux, d)
	return st, mux
}

func seed(t *testing.T, st *memory.Store, tk domain.Ticket) domain.Ticket {
	t.Helper()
	require.NoError(t, st.Create(t.Context(), &tk))
	return tk
}

func do(t *testing.T, h http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// element returns the <tag …>…</tag> element carrying data-ticket="id",
// or "" if the ticket is not in body.
func element(body, tag, id string) string {
	i := strings.Index(body, `data-ticket="`+id+`"`)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(body[:i], "<"+tag)
	end := strings.Index(body[i:], "</"+tag+">")
	return body[start : i+end]
}

// Regression: if the team, severity or needs-review filter is ignored, or
// the poll URL drops it, the agent's filtered view snaps back to every ticket.
func TestAgentQueueFilters(t *testing.T) {
	st, h := newAgentServer(t)
	netSev2 := seed(t, st, domain.Ticket{Title: "a", Status: domain.StatusOpen, Team: domain.TeamNetwork, Severity: domain.Sev2})
	netSev4 := seed(t, st, domain.Ticket{Title: "b", Status: domain.StatusOpen, Team: domain.TeamNetwork, Severity: domain.Sev4, NeedsReview: true})
	appSev2 := seed(t, st, domain.Ticket{Title: "c", Status: domain.StatusOpen, Team: domain.TeamApplications, Severity: domain.Sev2})

	cases := []struct {
		query string
		want  []string
	}{
		{"", []string{netSev2.ID, netSev4.ID, appSev2.ID}},
		{"team=network", []string{netSev2.ID, netSev4.ID}},
		{"severity=2", []string{netSev2.ID, appSev2.ID}},
		{"team=network&severity=2", []string{netSev2.ID}},
		{"needs_review=1", []string{netSev4.ID}},
	}
	for _, c := range cases {
		for _, path := range []string{"/agent", "/agent/queue"} {
			rec := do(t, h, http.MethodGet, path+"?"+c.query, nil)
			require.Equal(t, http.StatusOK, rec.Code, path+"?"+c.query)
			for _, id := range []string{netSev2.ID, netSev4.ID, appSev2.ID} {
				shown := element(rec.Body.String(), "tr", id) != ""
				assert.Equal(t, slices.Contains(c.want, id), shown, "%s?%s row %s", path, c.query, id)
			}
		}
	}

	rec := do(t, h, http.MethodGet, "/agent/queue?team=network&severity=2&needs_review=1", nil)
	poll := regexp.MustCompile(`id="agent-queue"[^>]*hx-get="([^"]+)"`).FindStringSubmatch(rec.Body.String())
	require.Len(t, poll, 2, "queue fragment has no poll URL")
	next, err := url.Parse(html.UnescapeString(poll[1]))
	require.NoError(t, err)
	assert.Equal(t, "network", next.Query().Get("team"))
	assert.Equal(t, "2", next.Query().Get("severity"))
	assert.Equal(t, "1", next.Query().Get("needs_review"))
	assert.NotEmpty(t, next.Query().Get("since"))

	assert.Equal(t, http.StatusBadRequest, do(t, h, http.MethodGet, "/agent/queue?team=netwrk", nil).Code)
	assert.Equal(t, http.StatusBadRequest, do(t, h, http.MethodGet, "/agent/queue?severity=9", nil).Code)
	assert.Equal(t, http.StatusBadRequest, do(t, h, http.MethodGet, "/agent/queue?needs_review=yes", nil).Code)
}

// Regression: if the flash ignores since, every row flashes every second
// (or none ever does), and the audience cannot see which ticket just
// landed; if triaging rows lose their grey "triaging…" state, a zero team
// and severity show as blanks.
func TestQueueSinceFlashesOnlyNewerRows(t *testing.T) {
	st, h := newAgentServer(t)
	old := seed(t, st, domain.Ticket{Title: "old", Status: domain.StatusOpen, Team: domain.TeamNetwork, Severity: domain.Sev2})
	fresh := seed(t, st, domain.Ticket{Title: "fresh", Status: domain.StatusOpen})
	time.Sleep(2 * time.Millisecond)
	since := time.Now()
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, st.Update(t.Context(), fresh.ID, func(tk *domain.Ticket) error {
		tk.Status = domain.StatusTriaging
		return nil
	}))

	body := do(t, h, http.MethodGet, "/agent/queue?since="+sinceValue(since), nil).Body.String()
	oldRow, freshRow := element(body, "tr", old.ID), element(body, "tr", fresh.ID)
	require.NotEmpty(t, oldRow)
	require.NotEmpty(t, freshRow)
	assert.NotContains(t, oldRow, web.FlashClass)
	assert.Contains(t, freshRow, web.FlashClass)
	assert.Contains(t, freshRow, web.TriagingClass)
	assert.Contains(t, freshRow, "triaging…")
	assert.NotContains(t, oldRow, web.TriagingClass)
}

// Regression: if the feed sorts oldest-first, drops a ticket's second run,
// or skips failed runs, the audience cannot watch jev decide, and a jev
// outage is invisible in the feed or the queue row (HLD 3.2 item 3, 4.6).
func TestAgentFeedNewestFirstWithErrorLine(t *testing.T) {
	st, h := newAgentServer(t)
	t0 := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	network := domain.Judgment{
		CompletedAt: t0.Add(time.Second),
		Latency:     640 * time.Millisecond,
		Usage:       domain.Usage{InputTokens: 607, OutputTokens: 146},
		Team:        domain.Choice{Option: string(domain.TeamNetwork), Confidence: 0.98},
		Scope:       domain.Choice{Option: string(domain.ScopeOneOffice)},
		Severity:    domain.Sev2,
	}
	a := seed(t, st, domain.Ticket{Title: "a", Status: domain.StatusOpen, Runs: []domain.Judgment{network}})
	b := seed(t, st, domain.Ticket{Title: "b", Status: domain.StatusOpen, Runs: []domain.Judgment{
		{CompletedAt: t0.Add(2 * time.Second), Error: "timeout"},
	}})
	c := seed(t, st, domain.Ticket{Title: "c", Status: domain.StatusOpen, Runs: []domain.Judgment{
		{CompletedAt: t0, Team: domain.Choice{Option: string(domain.TeamServiceDesk)}},
		{CompletedAt: t0.Add(3 * time.Second), Team: domain.Choice{Option: string(domain.TeamApplications)}},
	}})

	body := do(t, h, http.MethodGet, "/agent/feed", nil).Body.String()
	lines := regexp.MustCompile(`(?s)data-testid="agent-feed-line" data-ticket="([^"]+)">(.*?)</li>`).FindAllStringSubmatch(body, -1)
	var order []string
	text := map[string]string{}
	for _, l := range lines {
		order = append(order, l[1])
		text[l[1]] = l[2]
	}
	assert.Equal(t, []string{c.ID, b.ID, a.ID, c.ID}, order)
	assert.Equal(t, a.ID+" · Network 0.98 · one office · Sev 2 · 640 ms · 607 in / 146 out", text[a.ID])
	assert.Equal(t, b.ID+" · jev error: timeout · Service Desk · needs review", text[b.ID])

	// HLD 4.6: the error shows in the queue row too, not only the feed.
	row := element(do(t, h, http.MethodGet, "/agent/queue", nil).Body.String(), "tr", b.ID)
	assert.Contains(t, row, "jev error: timeout")
}

// Regression: if the Why panel reads the ticket's current values, or only
// the latest run, ticket 4's re-route is not explainable in the demo
// (HLD 3.2 item 4, scene 9).
func TestWhyPanelRendersTwoRuns(t *testing.T) {
	st, h := newAgentServer(t)
	tk := seed(t, st, domain.Ticket{Title: "Everything is broken", Status: domain.StatusOpen, Runs: []domain.Judgment{
		{
			Team:     domain.Choice{Option: string(domain.TeamServiceDesk), Probabilities: map[string]float64{"service_desk": 1}},
			Scope:    domain.Choice{Option: string(domain.ScopeUnclear), Probabilities: map[string]float64{"unclear": 0.99}},
			Severity: domain.Sev3, Rule: "unclear",
		},
		{
			Team:     domain.Choice{Option: string(domain.TeamApplications), Probabilities: map[string]float64{"applications": 0.96}},
			Scope:    domain.Choice{Option: string(domain.ScopeOneUser), Probabilities: map[string]float64{"one_user": 0.98}},
			Severity: domain.Sev3, Rule: "one-user-blocked", Blocked: 0.82,
		},
	}})

	rec := do(t, h, http.MethodGet, "/agent/tickets/"+tk.ID, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	runs := strings.Split(rec.Body.String(), `data-testid="agent-why-run"`)[1:]
	require.Len(t, runs, 2)
	assert.Contains(t, runs[0], "Scope unclear, so provisional")
	assert.Contains(t, runs[0], "width: 100%")
	assert.Contains(t, runs[1], "One user blocked")
	assert.Contains(t, runs[1], "width: 96%")
	assert.Contains(t, runs[1], "one user")
	assert.Contains(t, runs[1], "0.82")

	assert.Equal(t, http.StatusNotFound, do(t, h, http.MethodGet, "/agent/tickets/T-9999", nil).Code)
}

// Regression: if an action forgets its HLD 6 status move or its Override,
// a needs_info ticket stays stuck after the agent routes it, or the agent's
// change leaves no record; if an action leaves the needs-review flag on, a
// ticket a human has decided stays in the review queue (HLD 3.2 item 2).
func TestOverrideStatusMoves(t *testing.T) {
	cases := []struct {
		name       string
		action     string
		form       url.Values
		from       domain.Status
		wantStatus domain.Status
		want       domain.Override
	}{
		{"reassign needs_info", "reassign", url.Values{"team": {"applications"}}, domain.StatusNeedsInfo, domain.StatusOpen,
			domain.Override{Field: domain.OverrideTeam, From: "Service Desk", To: "Applications"}},
		{"reassign open", "reassign", url.Values{"team": {"applications"}}, domain.StatusOpen, domain.StatusOpen,
			domain.Override{Field: domain.OverrideTeam, From: "Service Desk", To: "Applications"}},
		{"severity needs_info", "severity", url.Values{"severity": {"1"}}, domain.StatusNeedsInfo, domain.StatusOpen,
			domain.Override{Field: domain.OverrideSeverity, From: "Sev 3", To: "Sev 1"}},
		{"close open", "close", url.Values{}, domain.StatusOpen, domain.StatusClosed,
			domain.Override{Field: domain.OverrideStatus, From: "open", To: "closed"}},
		{"close needs_info", "close", url.Values{}, domain.StatusNeedsInfo, domain.StatusClosed,
			domain.Override{Field: domain.OverrideStatus, From: "needs_info", To: "closed"}},
		{"reviewed open", "reviewed", url.Values{}, domain.StatusOpen, domain.StatusOpen,
			domain.Override{Field: "needs_review", From: "true", To: "false"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, h := newAgentServer(t)
			tk := seed(t, st, domain.Ticket{Status: c.from, Team: domain.TeamServiceDesk, Severity: domain.Sev3, NeedsReview: true})

			rec := do(t, h, http.MethodPost, "/agent/tickets/"+tk.ID+"/"+c.action, c.form)
			require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
			assert.Equal(t, "/agent/tickets/"+tk.ID, rec.Header().Get("Location"))

			got, err := st.Get(t.Context(), tk.ID)
			require.NoError(t, err)
			assert.Equal(t, c.wantStatus, got.Status)
			assert.False(t, got.NeedsReview, "an agent acted, so the flag clears")
			require.Len(t, got.Overrides, 1)
			o := got.Overrides[0]
			assert.False(t, o.At.IsZero())
			o.At = time.Time{}
			assert.Equal(t, c.want, o)
		})
	}
}

// Regression: if the 409 guard is dropped, an agent's change on a triaging
// ticket is silently overwritten when the background run lands (HLD 3.2
// item 5); if bad input is accepted, a ticket gets a team or severity that
// does not exist; if "Mark reviewed" is accepted on an unflagged ticket,
// the Override list records a change that did not happen. A refused action
// must also leave the needs-review flag as it was.
func TestOverrideRejected(t *testing.T) {
	cases := []struct {
		name      string
		status    domain.Status
		action    string
		form      url.Values
		want      int
		unflagged bool
	}{
		{"reassign triaging", domain.StatusTriaging, "reassign", url.Values{"team": {"network"}}, http.StatusConflict, false},
		{"severity triaging", domain.StatusTriaging, "severity", url.Values{"severity": {"1"}}, http.StatusConflict, false},
		{"close triaging", domain.StatusTriaging, "close", url.Values{}, http.StatusConflict, false},
		{"reassign closed", domain.StatusClosed, "reassign", url.Values{"team": {"network"}}, http.StatusConflict, false},
		{"unknown team", domain.StatusOpen, "reassign", url.Values{"team": {"netwrk"}}, http.StatusBadRequest, false},
		{"unknown severity", domain.StatusOpen, "severity", url.Values{"severity": {"5"}}, http.StatusBadRequest, false},
		{"reviewed triaging", domain.StatusTriaging, "reviewed", url.Values{}, http.StatusConflict, false},
		{"reviewed not flagged", domain.StatusOpen, "reviewed", url.Values{}, http.StatusConflict, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, h := newAgentServer(t)
			tk := seed(t, st, domain.Ticket{Status: c.status, NeedsReview: !c.unflagged})

			rec := do(t, h, http.MethodPost, "/agent/tickets/"+tk.ID+"/"+c.action, c.form)
			assert.Equal(t, c.want, rec.Code)

			got, err := st.Get(t.Context(), tk.ID)
			require.NoError(t, err)
			assert.Empty(t, got.Overrides)
			assert.Equal(t, c.status, got.Status)
			assert.Equal(t, !c.unflagged, got.NeedsReview)
			assert.Equal(t, tk.UpdatedAt, got.UpdatedAt)
		})
	}
}
