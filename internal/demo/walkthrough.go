// Package demo is the scripted browser walkthrough (HLD section 10). The
// same scenes run headed for `make demo` (cmd/demo) and headless in the e2e
// test. Elements are found only by their data-testid constants from package
// web, plus data-ticket to pick one ticket's row (HLD 3.3).
package demo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"

	"github.com/redscaresu/itsm/internal/demodata"
	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/web"
)

// The customer the walkthrough types in. The HLD gives no name or email.
const (
	customerName  = "Sam Taylor"
	customerEmail = "sam.taylor@example.gov.uk"
)

// Window geometry for the headed run: customer on the left half, agent on
// the right half of a 1920-pixel-wide display (HLD 10).
const (
	windowWidth  = 960
	windowHeight = 1000
)

// Scene 10 reassigns ticket 6 and closes ticket 1 (HLD 10).
const (
	reassignTicketN = 6
	reassignTo      = domain.TeamApplications
	closeTicketN    = 1
	// whyTicketN is the ticket whose Why panel scene 9 shows.
	whyTicketN = 4
)

// waitTimeout bounds each wait. A live ticket can sit triaging ~20 s when
// jev retries (HLD 4.6), so this leaves room over that.
const waitTimeout = 45 * time.Second

// Config is one walkthrough run.
type Config struct {
	BaseURL  string               // the itsm server, e.g. http://localhost:8080
	Tickets  []demodata.Ticket    // typed in order, one scene each
	Headless bool                 // false opens two visible windows
	SlowMo   time.Duration        // delay Playwright adds to every action
	Pause    time.Duration        // pause between scenes
	Logf     func(string, ...any) // scene progress; nil for none
}

// Outcome is what the customer screen showed once a ticket left triaging.
type Outcome struct {
	Team        string // team label, e.g. "Network"
	Severity    string // e.g. "Sev 2"
	NeedsReview bool
}

// TicketResult is one demo ticket's journey.
type TicketResult struct {
	N     int
	ID    string   // e.g. "T-0001"
	First Outcome  // after the first triage run
	After *Outcome // after the follow-up answers; nil if none were asked
}

// Row is one agent queue row as scene 10 leaves it.
type Row struct {
	Team, Severity, Status string
}

// Result is what the walkthrough saw. The headed run ignores it; the e2e
// test asserts on it.
type Result struct {
	Tickets   []TicketResult
	FeedLines int            // decision feed lines at scene 8
	WhyRuns   int            // runs in ticket 4's Why panel at scene 9
	Queue     map[string]Row // queue rows by ticket ID after scene 10
}

// Run plays the ten scenes. The driver and Chromium must already be
// installed (make demo-install).
func Run(ctx context.Context, cfg Config) (Result, error) {
	pw, err := playwright.Run()
	if err != nil {
		return Result{}, fmt.Errorf("demo: start playwright (run make demo-install): %w", err)
	}
	defer func() { _ = pw.Stop() }()

	customer, err := openWindow(pw, cfg, 0)
	if err != nil {
		return Result{}, err
	}
	agent, err := openWindow(pw, cfg, windowWidth)
	if err != nil {
		return Result{}, err
	}
	if !cfg.Headless {
		if err := fitHalves(customer, agent); err != nil {
			return Result{}, err
		}
	}
	w := &walk{cfg: cfg, customer: customer, agent: agent}
	return w.play(ctx)
}

// fitHalves resizes the windows to split the display side by side. The
// launch args assume a 1920-pixel display; macOS pulls a window that does
// not fit back on screen, so on a smaller display the two would overlap.
func fitHalves(pages ...playwright.Page) error {
	v, err := pages[0].Evaluate("[screen.availLeft, screen.availTop, screen.availWidth, screen.availHeight]")
	if err != nil {
		return fmt.Errorf("demo: read screen size: %w", err)
	}
	s, ok := v.([]any)
	if !ok || len(s) != 4 {
		return fmt.Errorf("demo: unexpected screen size %v", v)
	}
	left, top, width, height := s[0], s[1], s[2], s[3]
	lw, ok1 := left.(int)
	ww, ok2 := width.(int)
	if !ok1 || !ok2 {
		return fmt.Errorf("demo: unexpected screen size %v", v)
	}
	half := ww / len(pages)
	for i, p := range pages {
		cdp, err := p.Context().NewCDPSession(p)
		if err != nil {
			return fmt.Errorf("demo: open CDP session: %w", err)
		}
		win, err := cdp.Send("Browser.getWindowForTarget", nil)
		if err != nil {
			return fmt.Errorf("demo: find window: %w", err)
		}
		m, ok := win.(map[string]any)
		if !ok {
			return fmt.Errorf("demo: unexpected window %v", win)
		}
		bounds := map[string]any{"left": lw + i*half, "top": top, "width": half, "height": height}
		if _, err := cdp.Send("Browser.setWindowBounds", map[string]any{"windowId": m["windowId"], "bounds": bounds}); err != nil {
			return fmt.Errorf("demo: place window: %w", err)
		}
	}
	return nil
}

// openWindow launches its own Chromium, so each window can be placed with
// --window-position (HLD 10), and opens one page in it.
func openWindow(pw *playwright.Playwright, cfg Config, left int) (playwright.Page, error) {
	opts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(cfg.Headless),
		SlowMo:   playwright.Float(float64(cfg.SlowMo.Milliseconds())),
		Args: []string{
			"--window-position=" + strconv.Itoa(left) + ",0",
			"--window-size=" + strconv.Itoa(windowWidth) + "," + strconv.Itoa(windowHeight),
		},
	}
	browser, err := pw.Chromium.Launch(opts)
	if err != nil {
		return nil, fmt.Errorf("demo: launch chromium: %w", err)
	}
	// Headed, the page follows the window size; headless keeps Playwright's
	// fixed default viewport.
	page, err := browser.NewPage(playwright.BrowserNewPageOptions{NoViewport: playwright.Bool(!cfg.Headless)})
	if err != nil {
		return nil, fmt.Errorf("demo: open page: %w", err)
	}
	page.SetDefaultTimeout(float64(waitTimeout.Milliseconds()))
	return page, nil
}

// errStopped is returned when ctx ends between scenes.
var errStopped = errors.New("demo: stopped")

// walk holds the two windows through the scenes.
type walk struct {
	cfg      Config
	customer playwright.Page // left window
	agent    playwright.Page // right window
	ids      map[int]string  // demo ticket number -> ticket ID
}

func (w *walk) play(ctx context.Context) (Result, error) {
	var res Result
	w.ids = map[int]string{}

	w.logf("scene 1: both windows open, empty queue on the right")
	if err := w.sceneOpen(); err != nil {
		return res, err
	}
	for i, t := range w.cfg.Tickets {
		if err := w.pause(ctx); err != nil {
			return res, err
		}
		w.logf("scene %d: ticket %d, %q", i+2, t.N, t.Title)
		tr, err := w.sceneTicket(t)
		if err != nil {
			return res, fmt.Errorf("demo: ticket %d: %w", t.N, err)
		}
		res.Tickets = append(res.Tickets, tr)
	}

	steps := []struct {
		name string
		run  func() error
	}{
		{"scene 8: decision feed", func() (err error) { res.FeedLines, err = w.sceneFeed(); return err }},
		{"scene 9: ticket 4's Why panel", func() (err error) { res.WhyRuns, err = w.sceneWhy(); return err }},
		{"scene 10: reassign ticket 6, close ticket 1", func() (err error) { res.Queue, err = w.sceneActions(); return err }},
	}
	for _, s := range steps {
		if err := w.pause(ctx); err != nil {
			return res, err
		}
		w.logf("%s", s.name)
		if err := s.run(); err != nil {
			return res, fmt.Errorf("demo: %s: %w", s.name, err)
		}
	}
	return res, w.pause(ctx)
}

// sceneOpen is scene 1: the form on the left, the queue on the right.
func (w *walk) sceneOpen() error {
	if _, err := w.customer.Goto(w.cfg.BaseURL + "/"); err != nil {
		return fmt.Errorf("demo: open customer screen: %w", err)
	}
	if _, err := w.agent.Goto(w.cfg.BaseURL + "/agent"); err != nil {
		return fmt.Errorf("demo: open agent screen: %w", err)
	}
	return w.agent.GetByTestId(web.TestIDAgentQueue).WaitFor()
}

// sceneTicket is scenes 2-7: type one ticket on the left, watch it land on
// the right, and answer follow-ups if jev asks for them.
func (w *walk) sceneTicket(t demodata.Ticket) (TicketResult, error) {
	tr := TicketResult{N: t.N}
	c := w.customer
	if _, err := c.Goto(w.cfg.BaseURL + "/"); err != nil {
		return tr, err
	}
	fields := []struct{ id, value string }{
		{web.TestIDCustomerName, customerName},
		{web.TestIDCustomerEmail, customerEmail},
		{web.TestIDCustomerTitle, t.Title},
		{web.TestIDCustomerDescription, t.Description},
	}
	for _, f := range fields {
		if err := c.GetByTestId(f.id).Fill(f.value); err != nil {
			return tr, fmt.Errorf("fill %s: %w", f.id, err)
		}
	}
	if err := c.GetByTestId(web.TestIDCustomerSubmit).Click(); err != nil {
		return tr, fmt.Errorf("click %s: %w", web.TestIDCustomerSubmit, err)
	}
	id, err := c.GetByTestId(web.TestIDTicketID).TextContent()
	if err != nil {
		return tr, fmt.Errorf("read ticket ID: %w", err)
	}
	tr.ID = id
	w.ids[t.N] = id

	// The row lands on the right, grey, then flips (HLD 10).
	if err := w.agent.Locator(queueRow(id)).WaitFor(); err != nil {
		return tr, fmt.Errorf("row %s never reached the queue: %w", id, err)
	}
	if tr.First, err = w.waitTriaged(); err != nil {
		return tr, err
	}
	if t.Answers == nil {
		return tr, nil
	}
	asked, err := w.answerFollowUps(*t.Answers)
	if err != nil || !asked {
		return tr, err
	}
	after, err := w.waitTriaged()
	tr.After = &after
	return tr, err
}

// waitTriaged waits until the customer page is no longer triaging (the only
// wait HLD 10 allows), then reads what it shows.
func (w *walk) waitTriaged() (Outcome, error) {
	c := w.customer
	err := c.GetByTestId(web.TestIDCustomerTriaging).WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateHidden,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("still triaging: %w", err)
	}
	var o Outcome
	if o.Team, err = text(c.GetByTestId(web.TestIDTicketTeam)); err != nil {
		return o, err
	}
	if o.Severity, err = text(c.GetByTestId(web.TestIDTicketSeverity)); err != nil {
		return o, err
	}
	n, err := c.GetByTestId(web.TestIDNeedsReview).Count()
	o.NeedsReview = n > 0
	return o, err
}

// answerFollowUps types the demo answers into whichever follow-up boxes the
// page shows and submits them. It reports false if no follow-ups were asked,
// so live jev asking none does not stall the show.
func (w *walk) answerFollowUps(a demodata.Answers) (bool, error) {
	c := w.customer
	form := c.GetByTestId(web.TestIDCustomerFollowUpForm)
	if n, err := form.Count(); err != nil || n == 0 {
		return false, err
	}
	for _, id := range []domain.FollowUpID{domain.FollowUpLocation, domain.FollowUpService, domain.FollowUpImpact} {
		box := c.GetByTestId(web.FollowUpAnswerTestID(id))
		n, err := box.Count()
		if err != nil {
			return true, err
		}
		if n == 0 {
			continue
		}
		if err := box.Fill(a.For(id)); err != nil {
			return true, fmt.Errorf("answer %s: %w", id, err)
		}
	}
	if err := c.GetByTestId(web.TestIDCustomerFollowUpSubmit).Click(); err != nil {
		return true, fmt.Errorf("click %s: %w", web.TestIDCustomerFollowUpSubmit, err)
	}
	// The form goes once the answers are in and the page reloads.
	return true, form.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden})
}

// sceneFeed is scene 8: scroll the right window to the decision feed and
// wait for the last ticket's line. It returns the number of lines.
func (w *walk) sceneFeed() (int, error) {
	a := w.agent
	if err := a.GetByTestId(web.TestIDAgentFeed).ScrollIntoViewIfNeeded(); err != nil {
		return 0, err
	}
	if n := len(w.cfg.Tickets); n > 0 {
		last := w.ids[w.cfg.Tickets[n-1].N]
		if err := a.Locator(byTicket(web.TestIDAgentFeedLine, last)).WaitFor(); err != nil {
			return 0, fmt.Errorf("no feed line for %s: %w", last, err)
		}
	}
	return a.GetByTestId(web.TestIDAgentFeedLine).Count()
}

// sceneWhy is scene 9: ticket 4's Why panel, first and second runs side by
// side. It returns the number of runs shown.
func (w *walk) sceneWhy() (int, error) {
	a := w.agent
	if err := w.openAgentTicket(whyTicketN); err != nil {
		return 0, err
	}
	if err := a.GetByTestId(web.TestIDAgentWhyPanel).ScrollIntoViewIfNeeded(); err != nil {
		return 0, err
	}
	return a.GetByTestId(web.TestIDAgentWhyRun).Count()
}

// sceneActions is scene 10: reassign ticket 6 to Applications, close
// ticket 1, then show the queue. It returns every demo ticket's row.
func (w *walk) sceneActions() (map[string]Row, error) {
	a := w.agent
	if err := w.openAgentTicket(reassignTicketN); err != nil {
		return nil, err
	}
	values := []string{string(reassignTo)}
	if _, err := a.GetByTestId(web.TestIDAgentReassignTeam).SelectOption(playwright.SelectOptionValues{Values: &values}); err != nil {
		return nil, err
	}
	if err := w.submitAction(web.TestIDAgentReassignSubmit); err != nil {
		return nil, err
	}
	if err := w.openAgentTicket(closeTicketN); err != nil {
		return nil, err
	}
	if err := w.submitAction(web.TestIDAgentCloseSubmit); err != nil {
		return nil, err
	}
	if _, err := a.Goto(w.cfg.BaseURL + "/agent"); err != nil {
		return nil, err
	}
	return w.readQueue()
}

// openAgentTicket opens demo ticket n's agent page in the right window.
func (w *walk) openAgentTicket(n int) error {
	id, ok := w.ids[n]
	if !ok {
		return fmt.Errorf("demo ticket %d was not raised", n)
	}
	if _, err := w.agent.Goto(w.cfg.BaseURL + "/agent/tickets/" + id); err != nil {
		return err
	}
	return w.agent.GetByTestId(web.TestIDTicketID).WaitFor()
}

// submitAction clicks an agent action and waits for the page it redirects
// to, which lists the change the action recorded.
func (w *walk) submitAction(testID string) error {
	if err := w.agent.GetByTestId(testID).Click(); err != nil {
		return fmt.Errorf("click %s: %w", testID, err)
	}
	if err := w.agent.GetByTestId(web.TestIDAgentOverride).First().WaitFor(); err != nil {
		return fmt.Errorf("%s recorded no change: %w", testID, err)
	}
	return nil
}

// readQueue reads each demo ticket's row from the agent queue.
func (w *walk) readQueue() (map[string]Row, error) {
	rows := make(map[string]Row, len(w.ids))
	for _, id := range w.ids {
		row := w.agent.Locator(queueRow(id))
		var r Row
		var err error
		if r.Team, err = text(row.GetByTestId(web.TestIDTicketTeam)); err != nil {
			return nil, err
		}
		if r.Severity, err = text(row.GetByTestId(web.TestIDTicketSeverity)); err != nil {
			return nil, err
		}
		if r.Status, err = text(row.GetByTestId(web.TestIDTicketStatus)); err != nil {
			return nil, err
		}
		rows[id] = r
	}
	return rows, nil
}

// pause holds a scene on screen so the audience can read it.
func (w *walk) pause(ctx context.Context) error {
	t := time.NewTimer(w.cfg.Pause)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return errStopped
	case <-t.C:
		return nil
	}
}

func (w *walk) logf(format string, args ...any) {
	if w.cfg.Logf != nil {
		w.cfg.Logf(format, args...)
	}
}

// queueRow selects one ticket's row in the agent queue.
func queueRow(id string) string {
	return byTicket(web.TestIDAgentQueueRow, id)
}

// byTicket selects the element with a test ID that belongs to one ticket.
func byTicket(testID, ticketID string) string {
	return fmt.Sprintf("[data-testid=%q][data-ticket=%q]", testID, ticketID)
}

// text reads an element's text without surrounding white space.
func text(l playwright.Locator) (string, error) {
	s, err := l.TextContent()
	return strings.TrimSpace(s), err
}
