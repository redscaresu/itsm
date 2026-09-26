//go:build live

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/demodata"
	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/jev"
	"github.com/redscaresu/itsm/internal/store/memory"
	"github.com/redscaresu/itsm/internal/triage"
)

// liveRunTimeout covers one jev call with its retry (HLD 4.6: ~20 s worst case).
const liveRunTimeout = 30 * time.Second

// Regression: jev's behaviour drifts, or our questions stop producing usable
// answers, and a demo ticket lands on the wrong team or severity in front of
// an audience (HLD 4.5, section 9 live row). Runs only with -tags live and
// TYPESAFE_API_KEY set; it calls the real jev API.
func TestLiveDemoTickets(t *testing.T) {
	if os.Getenv("TYPESAFE_API_KEY") == "" {
		t.Skip("TYPESAFE_API_KEY not set; skipping the live jev smoke test")
	}
	cfg, err := loadConfig(os.Getenv)
	require.NoError(t, err)
	tickets, err := demodata.Load(demoFile)
	require.NoError(t, err)

	store := memory.New()
	triager := jev.NewTriager(jev.NewClient(jev.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey}))
	runner := triage.NewRunner(t.Context(), triage.Deps{
		Store: store, Triager: triager, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, tk := range tickets {
		t.Run(fmt.Sprintf("ticket_%d", tk.N), func(t *testing.T) {
			ticket := domain.Ticket{Title: tk.Title, Description: tk.Description, Status: domain.StatusTriaging}
			require.NoError(t, store.Create(t.Context(), &ticket))
			got := triageOnce(t, store, runner, ticket.ID)
			assertRun(t, got.Runs[0], tk.Jev[0])
			assert.Equal(t, tk.Expect.Team, got.Team, "team")
			assert.Equal(t, tk.Expect.Severity, got.Severity, "severity")
			assert.Equal(t, tk.Expect.Status, got.Status, "status")
			assert.Equal(t, tk.Expect.NeedsReview, got.NeedsReview, "needs review")
			if tk.ExpectAfter == nil || got.Status != domain.StatusNeedsInfo {
				return
			}

			// The customer answers the follow-ups; the second run re-routes.
			require.NoError(t, store.Update(t.Context(), ticket.ID, func(tt *domain.Ticket) error {
				for i := range tt.FollowUps {
					tt.FollowUps[i].Answer = tk.Answers.For(tt.FollowUps[i].ID)
					tt.FollowUps[i].AnsweredAt = time.Now()
				}
				tt.Status = domain.StatusTriaging
				return nil
			}))
			got = triageOnce(t, store, runner, ticket.ID)
			require.Len(t, got.Runs, 2)
			assertRun(t, got.Runs[1], tk.Jev[1])
			assert.Equal(t, tk.ExpectAfter.Team, got.Team, "team after follow-up")
			assert.Equal(t, tk.ExpectAfter.Severity, got.Severity, "severity after follow-up")
			assert.Equal(t, domain.StatusOpen, got.Status, "status after follow-up")
		})
	}
}

// triageOnce runs one triage pass on the ticket and returns it afterwards.
func triageOnce(t *testing.T, store domain.Store, runner *triage.Runner, id string) domain.Ticket {
	t.Helper()
	runner.Start(id)
	ctx, cancel := context.WithTimeout(t.Context(), liveRunTimeout)
	defer cancel()
	require.NoError(t, runner.Wait(ctx))
	got, err := store.Get(t.Context(), id)
	require.NoError(t, err)
	require.NotEmpty(t, got.Runs)
	return got
}

// assertRun checks one live run's team, kind and scope choices against the
// HLD 4.5 row. It does not compare probabilities; it logs every answer so a
// drift can be reported with the numbers.
func assertRun(t *testing.T, got domain.Judgment, want demodata.Run) {
	t.Helper()
	t.Logf("model %s: team %s (%.2f), kind %s (%.2f), scope %s (%.2f), blocked %.2f, loc %.2f, svc %.2f, rule %s, %s",
		got.Model, got.Team.Option, got.Team.Confidence, got.Kind.Option, got.Kind.Confidence,
		got.Scope.Option, got.Scope.Confidence, got.Blocked, got.StatesLocation, got.StatesService, got.Rule, got.Latency)
	require.Empty(t, got.Error, "jev call failed")
	assert.Equal(t, string(want.Team), got.Team.Option, "jev team")
	assert.Equal(t, string(want.Kind), got.Kind.Option, "jev kind")
	assert.Equal(t, string(want.Scope), got.Scope.Option, "jev scope")
}
