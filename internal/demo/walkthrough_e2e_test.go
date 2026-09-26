//go:build e2e

package demo

import (
	"bytes"
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/demodata"
	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
)

// e2eTimeout bounds the whole headless walkthrough.
const e2eTimeout = 3 * time.Minute

// Regression: if someone renames or removes a form field, button or test ID
// the walkthrough uses, or a demo ticket stops landing on its team and
// severity, the demo dies live in front of an audience (HLD 9). This runs
// all ten scenes headless against the real binary and a scripted fake jev.
func TestWalkthroughHeadless(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), e2eTimeout)
	defer cancel()

	tickets, err := demodata.Load(filepath.Join("..", "..", demodata.File))
	require.NoError(t, err)
	fake := jevfake.New()
	t.Cleanup(fake.Close)
	fake.Script(demodata.Script(tickets))

	base := startItsm(ctx, t, fake.URL)
	res, err := Run(ctx, Config{BaseURL: base, Tickets: tickets, Headless: true, Logf: t.Logf})
	require.NoError(t, err)

	require.Len(t, res.Tickets, len(tickets))
	ids := map[int]string{}
	runs := 0
	for i, want := range tickets {
		got := res.Tickets[i]
		ids[want.N] = got.ID
		runs += len(want.Jev)
		assert.Equal(t, want.Expect.Team.Label(), got.First.Team, "ticket %d team", want.N)
		assert.Equal(t, want.Expect.Severity.String(), got.First.Severity, "ticket %d severity", want.N)
		assert.Equal(t, want.Expect.NeedsReview, got.First.NeedsReview, "ticket %d needs review", want.N)
		if want.ExpectAfter == nil {
			assert.Nil(t, got.After, "ticket %d was asked follow-ups", want.N)
			continue
		}
		if assert.NotNil(t, got.After, "ticket %d: follow-ups were not answered", want.N) {
			assert.Equal(t, want.ExpectAfter.Team.Label(), got.After.Team, "ticket %d team after answers", want.N)
			assert.Equal(t, want.ExpectAfter.Severity.String(), got.After.Severity, "ticket %d severity after answers", want.N)
		}
	}

	assert.Equal(t, runs, res.FeedLines, "scene 8: one feed line per triage run")
	assert.Equal(t, 2, res.WhyRuns, "scene 9: ticket 4's Why panel shows both runs")
	assert.Equal(t, reassignTo.Label(), res.Queue[ids[reassignTicketN]].Team, "scene 10: ticket 6 reassigned")
	assert.Equal(t, string(domain.StatusClosed), res.Queue[ids[closeTicketN]].Status, "scene 10: ticket 1 closed")
}

// startItsm builds the itsm binary, runs it against the fake jev and
// returns its base URL. The server is stopped when the test ends.
func startItsm(ctx context.Context, t *testing.T, jevURL string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "itsm")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "../../cmd/itsm")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build: %s", out)

	port := freePort(t)
	base := "http://127.0.0.1:" + port
	var logs bytes.Buffer
	// Registered first so it runs last, after Stop has collected the output.
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("itsm output:\n%s", logs.String())
		}
	})
	srv, err := StartServer(ctx, bin, base, []string{
		"PORT=" + port,
		"TYPESAFE_BASE_URL=" + jevURL,
		"TYPESAFE_API_KEY=e2e-fake-key",
	}, &logs)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, srv.Stop(), "itsm did not shut down cleanly") })
	return base
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return strconv.Itoa(port)
}
