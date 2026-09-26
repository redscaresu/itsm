package triage

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
)

// Deps is what a Runner needs.
type Deps struct {
	Store   domain.Store
	Triager domain.Triager
	Logger  *slog.Logger
}

// Runner triages tickets in the background: one goroutine per Start, no
// queue, no worker pool (HLD section 7).
type Runner struct {
	base context.Context
	deps Deps
	now  func() time.Time
	wg   sync.WaitGroup
}

var _ domain.Runner = (*Runner)(nil)

// NewRunner returns a Runner whose runs use base as their context. base must
// outlive the request that called Start; run() cancels it only after Wait
// returns or the shutdown drain expires.
func NewRunner(base context.Context, deps Deps) *Runner {
	return &Runner{base: base, deps: deps, now: time.Now}
}

// Start begins a triage run for the ticket and returns at once.
func (r *Runner) Start(ticketID string) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.run(ticketID)
	}()
}

// Wait blocks until every in-flight run has finished, or returns ctx's
// error if ctx ends first.
func (r *Runner) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is one triage run: load, ask jev, decide, then one Store.Update.
func (r *Runner) run(id string) {
	ctx := r.base
	log := r.deps.Logger.With("ticket", id)
	t, err := r.deps.Store.Get(ctx, id)
	if err != nil {
		log.Error("triage: load ticket", "err", err)
		return
	}
	j, err := r.deps.Triager.Triage(ctx, t)
	var apply func(*domain.Ticket) error
	if err != nil {
		log.Warn("triage: jev call failed", "err", err)
		apply = r.failure(err)
	} else {
		apply = r.success(j, len(t.FollowUps) == 0)
	}
	if err := r.deps.Store.Update(ctx, id, apply); err != nil {
		log.Error("triage: save run", "err", err)
	}
}

// success records a completed run and applies its Outcome.
func (r *Runner) success(j domain.Judgment, firstRound bool) func(*domain.Ticket) error {
	o := Decide(j, firstRound)
	j.Severity, j.Rule, j.NeedsReview = o.Severity, o.Rule, o.NeedsReview
	if j.CompletedAt.IsZero() {
		j.CompletedAt = r.now()
	}
	return func(t *domain.Ticket) error {
		t.Runs = append(t.Runs, j)
		t.Team, t.Severity, t.NeedsReview, t.Status = o.Team, o.Severity, o.NeedsReview, o.Status
		if len(o.FollowUps) > 0 {
			t.FollowUps = o.FollowUps
		}
		return nil
	}
}

// failure records a failed jev call and applies the HLD 4.6 failure values:
// Service Desk, Sev 3, needs review, open. The ticket is never left
// triaging and never lost.
func (r *Runner) failure(err error) func(*domain.Ticket) error {
	j := domain.Judgment{
		CompletedAt: r.now(),
		Severity:    domain.Sev3,
		NeedsReview: true,
		Error:       err.Error(),
	}
	return func(t *domain.Ticket) error {
		t.Runs = append(t.Runs, j)
		t.Team, t.Severity, t.NeedsReview, t.Status = domain.TeamServiceDesk, domain.Sev3, true, domain.StatusOpen
		return nil
	}
}
