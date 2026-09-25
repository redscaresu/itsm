package triage

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/store/memory"
)

const (
	eventually = 2 * time.Second
	tick       = 5 * time.Millisecond
)

// triagerFunc is a canned Triager.
type triagerFunc func(context.Context, domain.Ticket) (domain.Judgment, error)

func (f triagerFunc) Triage(ctx context.Context, t domain.Ticket) (domain.Judgment, error) {
	return f(ctx, t)
}

func answer(j domain.Judgment) triagerFunc {
	return func(context.Context, domain.Ticket) (domain.Judgment, error) { return j, nil }
}

// setup saves one triaging ticket and returns a runner over it.
func setup(t *testing.T, tr domain.Triager) (*Runner, *memory.Store, string) {
	t.Helper()
	st := memory.New()
	tk := &domain.Ticket{Title: "Everything is broken", Status: domain.StatusTriaging}
	require.NoError(t, st.Create(context.Background(), tk))
	r := NewRunner(context.Background(), Deps{Store: st, Triager: tr, Logger: slog.New(slog.DiscardHandler)})
	return r, st, tk.ID
}

// waitStatus waits until the ticket reaches want and returns it.
func waitStatus(t *testing.T, st *memory.Store, id string, want domain.Status) domain.Ticket {
	t.Helper()
	require.Eventually(t, func() bool {
		got, err := st.Get(context.Background(), id)
		return err == nil && got.Status == want
	}, eventually, tick)
	got, err := st.Get(context.Background(), id)
	require.NoError(t, err)
	return got
}

// Catches: a first run whose trigger fires does not write the follow-up
// questions, or does not move the ticket to needs_info.
func TestRunnerFirstRunNeedsInfo(t *testing.T) {
	j := run(1, domain.ScopeUnclear, 0.03, 0.03)
	j.Team.Option = string(domain.TeamServiceDesk)
	r, st, id := setup(t, answer(j))

	r.Start(id)
	got := waitStatus(t, st, id, domain.StatusNeedsInfo)

	assert.Equal(t, []domain.FollowUpID{domain.FollowUpLocation, domain.FollowUpService, domain.FollowUpImpact}, ids(got.FollowUps))
	for _, f := range got.FollowUps {
		assert.Empty(t, f.Answer)
	}
	assert.Equal(t, domain.TeamServiceDesk, got.Team)
	assert.Equal(t, domain.Sev3, got.Severity)
	assert.True(t, got.NeedsReview)
	require.Len(t, got.Runs, 1)
	assert.Equal(t, RuleUnclear, got.Runs[0].Rule)
	assert.Equal(t, domain.Sev3, got.Runs[0].Severity)
	assert.False(t, got.Runs[0].CompletedAt.IsZero())
}

// Catches: a second run (follow-ups already answered) asks again or
// overwrites the customer's answers, instead of moving to open. jev still
// says "unclear" here, so only the one-round rule keeps it out of needs_info.
func TestRunnerSecondRunOpens(t *testing.T) {
	r, st, id := setup(t, answer(run(0.96, domain.ScopeUnclear, 0.03, 0.03)))
	answered := []domain.FollowUp{{ID: domain.FollowUpLocation, Question: QuestionLocation, Answer: "Bristol office"}}
	require.NoError(t, st.Update(context.Background(), id, func(tk *domain.Ticket) error {
		tk.FollowUps = answered
		tk.Runs = []domain.Judgment{{Rule: RuleUnclear}}
		return nil
	}))

	r.Start(id)
	got := waitStatus(t, st, id, domain.StatusOpen)

	assert.Equal(t, answered, got.FollowUps)
	assert.Equal(t, domain.TeamApplications, got.Team)
	assert.Equal(t, domain.Sev3, got.Severity)
	assert.True(t, got.NeedsReview, "trigger fired with no round left: an agent must look")
	require.Len(t, got.Runs, 2, "the first run stays for the side-by-side Why panel")
	assert.Equal(t, RuleUnclear, got.Runs[1].Rule)
}

// Catches: a confident first run is interrupted with follow-ups.
func TestRunnerConfidentRunOpens(t *testing.T) {
	r, st, id := setup(t, answer(run(1, domain.ScopeOneOffice, 0.07, 0.9)))

	r.Start(id)
	got := waitStatus(t, st, id, domain.StatusOpen)

	assert.Empty(t, got.FollowUps)
	assert.Equal(t, domain.Sev2, got.Severity)
}

// Catches: someone changes the failure path and a ticket is left triaging,
// or loses its run record, when jev is down (HLD 4.6).
func TestRunnerTriagerError(t *testing.T) {
	r, st, id := setup(t, triagerFunc(func(context.Context, domain.Ticket) (domain.Judgment, error) {
		return domain.Judgment{}, errors.New("timeout")
	}))

	r.Start(id)
	got := waitStatus(t, st, id, domain.StatusOpen)

	assert.Equal(t, domain.TeamServiceDesk, got.Team)
	assert.Equal(t, domain.Sev3, got.Severity)
	assert.True(t, got.NeedsReview)
	require.Len(t, got.Runs, 1)
	assert.Equal(t, "timeout", got.Runs[0].Error)
	assert.True(t, got.Runs[0].NeedsReview)
	assert.False(t, got.Runs[0].CompletedAt.IsZero())
}

// Catches: Wait returns before in-flight runs finish (shutdown drops a
// ticket mid-triage), or blocks past an expired shutdown deadline.
func TestRunnerWait(t *testing.T) {
	release := make(chan struct{})
	r, st, id := setup(t, triagerFunc(func(context.Context, domain.Ticket) (domain.Judgment, error) {
		<-release
		return run(1, domain.ScopeOneOffice, 1, 1), nil
	}))
	r.Start(id)

	expired, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, r.Wait(expired), context.DeadlineExceeded)

	close(release)
	require.NoError(t, r.Wait(context.Background()))
	got, err := st.Get(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusOpen, got.Status, "Wait returned before the run saved")
}
