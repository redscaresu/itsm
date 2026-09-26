package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/demodata"
	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
	"github.com/redscaresu/itsm/internal/web"
)

// End-to-end tests: each calls run() and talks to it over real HTTP (HLD 7, 9).

// Regression: if the key check is dropped, the server starts without a key
// and every triage run fails at the first jev call instead of at startup.
func TestRunFailsWithoutAPIKey(t *testing.T) {
	// Bounded, so a server that wrongly starts returns nil instead of hanging.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err := run(ctx, io.Discard, nil, env(map[string]string{"PORT": freePort(t)}))
	require.ErrorContains(t, err, "TYPESAFE_API_KEY")
}

// Regression: if /healthz, the /static/ mount or the /assets/ mount breaks,
// the demo loses its readiness check, htmx, or the GOV.UK fonts; and if
// shutdown stops returning cleanly, run() hangs or errors on Ctrl-C.
func TestRunServesHealthzAndAssets(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, io.Discard, nil, env(map[string]string{"TYPESAFE_API_KEY": "x", "PORT": port}))
	}()
	base := "http://127.0.0.1:" + port
	waitForReady(t, base+"/healthz")

	assertOK(t, base+"/static/htmx.min.js")

	css, err := fs.Glob(web.Static(), "govuk/govuk-frontend-*.min.css")
	require.NoError(t, err)
	require.Len(t, css, 1)
	body := assertOK(t, base+"/static/"+css[0])
	font := regexp.MustCompile(`/assets/fonts/[^)"']+`).FindString(body)
	require.NotEmpty(t, font, "govuk CSS names no font")
	assertOK(t, base+font)

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// freePort asks the OS for a free port. Another process could take it
// before run() binds; that is rare enough for a test.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	require.NoError(t, l.Close())
	return port
}

// waitForReady polls url until it answers 200 or the test times out.
func waitForReady(t *testing.T, url string) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)
}

func assertOK(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, url)
	return string(b)
}

// Regression: a handler stops redirecting, a status transition breaks, an
// agent action is allowed while triaging, an override isn't recorded, or a
// polling partial 500s or flashes the wrong rows (HLD section 9, server row).
// Demo ticket 4 walks the whole lifecycle; demo ticket 1 is the "newer" row.
func TestTicketLifecycle(t *testing.T) {
	tickets, err := demodata.Load(demoFile)
	require.NoError(t, err)
	vague, leeds := demoTicket(t, tickets, 4), demoTicket(t, tickets, 1)
	jevGate := startFakeJev(t, tickets)
	base, stop := startServer(t, jevGate.url)

	// Create: saved as triaging before jev answers, then needs_info.
	id := createTicket(t, base, vague)
	assert.Equal(t, string(domain.StatusTriaging), statusPanel(t, base, id).status)
	jevGate.letOneThrough(t)
	p := waitForStatus(t, base, id, domain.StatusNeedsInfo)
	assert.Equal(t, vague.Expect.Team.Label(), p.team)
	assert.Equal(t, vague.Expect.Severity.String(), p.severity)
	assert.True(t, p.needsReview, "a ticket triage could not decide is flagged")
	for _, f := range []domain.FollowUpID{domain.FollowUpLocation, domain.FollowUpService, domain.FollowUpImpact} {
		assert.Contains(t, p.body, testID(web.FollowUpAnswerTestID(f)), "follow-up box %q", f)
	}

	// A second ticket lands after since; only it flashes in the queue.
	since := time.Now()
	other := createTicket(t, base, leeds)
	jevGate.letOneThrough(t)
	waitForStatus(t, base, other, leeds.Expect.Status)
	flash := queueFlash(t, base, since)
	assert.Equal(t, map[string]bool{id: false, other: true}, flash)

	// Follow-up answers send the ticket back to triaging; agents must wait.
	answers := url.Values{}
	for _, f := range []domain.FollowUpID{domain.FollowUpLocation, domain.FollowUpService, domain.FollowUpImpact} {
		answers.Set(string(f), vague.Answers.For(f))
	}
	assertRedirect(t, base+"/tickets/"+id+"/follow-up", answers, "/tickets/"+id)
	assert.Equal(t, string(domain.StatusTriaging), statusPanel(t, base, id).status)
	code, _ := post(t, base+"/agent/tickets/"+id+"/reassign", url.Values{"team": {string(domain.TeamNetwork)}})
	assert.Equal(t, http.StatusConflict, code, "reassign while triaging")

	// The second run always opens the ticket, re-routed.
	jevGate.letOneThrough(t)
	p = waitForStatus(t, base, id, domain.StatusOpen)
	assert.Equal(t, vague.ExpectAfter.Team.Label(), p.team)
	assert.Equal(t, vague.ExpectAfter.Severity.String(), p.severity)
	assert.Equal(t, 2, strings.Count(assertOK(t, base+"/agent/feed"), feedLineMarker(id)), "feed lists both runs")

	// Agent reassigns, then closes; both are recorded as overrides.
	agentPage := "/agent/tickets/" + id
	assertRedirect(t, base+agentPage+"/reassign", url.Values{"team": {string(domain.TeamNetwork)}}, agentPage)
	assert.Equal(t, domain.TeamNetwork.Label(), statusPanel(t, base, id).team)
	assertRedirect(t, base+agentPage+"/close", nil, agentPage)
	assert.Equal(t, string(domain.StatusClosed), statusPanel(t, base, id).status)
	assert.Equal(t, 2, strings.Count(assertOK(t, base+agentPage), testID(web.TestIDAgentOverride)), "overrides recorded")

	require.NoError(t, stop())
}

// Regression: if run() stops waiting for in-flight triage runs on shutdown,
// or cancels their context at the signal, a ticket raised just before Ctrl-C
// is cut off mid-call instead of being finished (HLD section 7).
func TestShutdownWaitsForTriageRuns(t *testing.T) {
	tickets, err := demodata.Load(demoFile)
	require.NoError(t, err)
	jevGate := startFakeJev(t, tickets)
	base, stop := startServer(t, jevGate.url)
	createTicket(t, base, demoTicket(t, tickets, 1))

	stopped := make(chan error, 1)
	go func() { stopped <- stop() }()
	select {
	case err := <-stopped:
		t.Fatalf("run returned (%v) while a triage run was still in flight", err)
	case <-time.After(shutdownGrace):
	}
	jevGate.letOneThrough(t)
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(runExitTimeout):
		t.Fatal("run did not return after the triage run finished")
	}
}

const (
	// shutdownGrace is how long a correct run() must keep waiting for an
	// in-flight triage run before the test lets it finish.
	shutdownGrace = 300 * time.Millisecond
	// runExitTimeout is longer than run()'s 10 s shutdown drain.
	runExitTimeout = 15 * time.Second
	pollTimeout    = 5 * time.Second
	pollInterval   = 20 * time.Millisecond
)

var demoFile = filepath.Join("..", "..", demodata.File)

// client does not follow redirects, so tests can assert on them.
var client = &http.Client{
	Timeout:       pollTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func demoTicket(t *testing.T, tickets []demodata.Ticket, n int) demodata.Ticket {
	t.Helper()
	for _, tk := range tickets {
		if tk.N == n {
			return tk
		}
	}
	t.Fatalf("no demo ticket %d", n)
	return demodata.Ticket{}
}

// jevGate sits between the server and the fake jev. It holds every jev call
// until the test lets one through, so a ticket stays triaging for exactly as
// long as the test needs.
type jevGate struct {
	url     string
	release chan struct{}
}

// startFakeJev starts a fake jev scripted from the demo tickets, behind a gate.
func startFakeJev(t *testing.T, tickets []demodata.Ticket) *jevGate {
	t.Helper()
	fake := jevfake.New()
	fake.Script(demodata.Script(tickets))
	target, err := url.Parse(fake.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorLog = log.New(io.Discard, "", 0)

	g := &jevGate{release: make(chan struct{})}
	closed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-g.release:
			proxy.ServeHTTP(w, r)
		case <-closed:
		case <-r.Context().Done():
		}
	}))
	g.url = srv.URL
	t.Cleanup(func() {
		close(closed)
		srv.Close()
		fake.Close()
	})
	return g
}

// letOneThrough releases one waiting jev call, failing if none arrives.
func (g *jevGate) letOneThrough(t *testing.T) {
	t.Helper()
	select {
	case g.release <- struct{}{}:
	case <-time.After(pollTimeout):
		t.Fatal("no jev call arrived at the gate")
	}
}

// startServer runs the real program against jevURL and returns its base URL
// and a stop func that cancels run() and returns its error.
func startServer(t *testing.T, jevURL string) (string, func() error) {
	t.Helper()
	port := freePort(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, io.Discard, nil, env(map[string]string{
			"TYPESAFE_API_KEY": "test-key", "TYPESAFE_BASE_URL": jevURL, "PORT": port,
		}))
	}()
	t.Cleanup(cancel)
	base := "http://127.0.0.1:" + port
	waitForReady(t, base+"/healthz")
	stop := sync.OnceValue(func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(runExitTimeout):
			return errors.New("run did not return after cancel")
		}
	})
	return base, stop
}

// post submits a form and returns the status code and Location header.
func post(t *testing.T, target string, form url.Values) (int, string) {
	t.Helper()
	resp, err := client.PostForm(target, form)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

func assertRedirect(t *testing.T, target string, form url.Values, wantLocation string) {
	t.Helper()
	code, loc := post(t, target, form)
	assert.Equal(t, http.StatusSeeOther, code, target)
	assert.Equal(t, wantLocation, loc, target)
}

// createTicket raises a demo ticket on the customer form and returns its ID.
func createTicket(t *testing.T, base string, tk demodata.Ticket) string {
	t.Helper()
	code, loc := post(t, base+"/tickets", url.Values{
		"name": {"Demo Customer"}, "email": {"demo@example.com"},
		"title": {tk.Title}, "description": {tk.Description},
	})
	require.Equal(t, http.StatusSeeOther, code)
	require.True(t, strings.HasPrefix(loc, "/tickets/T-"), "redirect to %q", loc)
	return path.Base(loc)
}

// panel is what the customer's status partial shows.
type panel struct {
	status, team, severity string
	needsReview            bool
	body                   string
}

var dataStatus = regexp.MustCompile(`data-status="([a-z_]+)"`)

func statusPanel(t *testing.T, base, id string) panel {
	t.Helper()
	return parsePanel(assertOK(t, base+"/tickets/"+id+"/status"))
}

func parsePanel(body string) panel {
	p := panel{
		team:        testIDText(body, web.TestIDTicketTeam),
		severity:    testIDText(body, web.TestIDTicketSeverity),
		needsReview: strings.Contains(body, testID(web.TestIDNeedsReview)),
		body:        body,
	}
	if m := dataStatus.FindStringSubmatch(body); m != nil {
		p.status = m[1]
	}
	return p
}

// waitForStatus polls the status partial until the ticket reaches want.
func waitForStatus(t *testing.T, base, id string, want domain.Status) panel {
	t.Helper()
	var p panel
	require.Eventually(t, func() bool {
		resp, err := client.Get(base + "/tickets/" + id + "/status")
		if err != nil {
			return false
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			return false
		}
		p = parsePanel(string(b))
		return p.status == string(want)
	}, pollTimeout, pollInterval, "ticket %s never reached %s", id, want)
	return p
}

var queueRow = regexp.MustCompile(`<tr class="([^"]*)" ` + regexp.QuoteMeta(testID(web.TestIDAgentQueueRow)) + ` data-ticket="([^"]+)"`)

// queueFlash polls the agent queue once with since and reports, per ticket,
// whether its row flashes.
func queueFlash(t *testing.T, base string, since time.Time) map[string]bool {
	t.Helper()
	body := assertOK(t, base+"/agent/queue?since="+strconv.FormatInt(since.UnixNano(), 10))
	out := map[string]bool{}
	for _, m := range queueRow.FindAllStringSubmatch(body, -1) {
		out[m[2]] = slices.Contains(strings.Fields(m[1]), web.FlashClass)
	}
	return out
}

func testID(id string) string { return `data-testid="` + id + `"` }

// testIDText is the text directly inside the element with this test ID.
func testIDText(body, id string) string {
	m := regexp.MustCompile(regexp.QuoteMeta(testID(id)) + `>([^<]*)<`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func feedLineMarker(id string) string {
	return testID(web.TestIDAgentFeedLine) + ` data-ticket="` + id + `"`
}
