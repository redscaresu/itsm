package domain

import (
	"context"
	"errors"
)

// ErrNotFound is returned by a Store when no ticket has the given ID.
var ErrNotFound = errors.New("ticket not found")

// Store keeps tickets (HLD section 6).
type Store interface {
	// Create saves a new ticket. It assigns t.ID (e.g. "T-0001") and sets
	// CreatedAt and UpdatedAt, writing them back into t.
	Create(ctx context.Context, t *Ticket) error
	// Get returns a copy of the ticket, or ErrNotFound.
	Get(ctx context.Context, id string) (Ticket, error)
	// List returns copies of all tickets, oldest first.
	List(ctx context.Context) ([]Ticket, error)
	// Update runs fn on a copy of the ticket under the store's lock. If fn
	// returns nil the copy is saved and UpdatedAt is bumped; if fn returns an
	// error nothing changes and that error is returned. Each writer changes
	// only its own fields, so a slow triage run cannot overwrite an agent's
	// override made in between.
	Update(ctx context.Context, id string, fn func(*Ticket) error) error
}

// Triager asks jev about one ticket. Only the runner calls it; the server
// never sees it (HLD section 7).
type Triager interface {
	Triage(ctx context.Context, t Ticket) (Judgment, error)
}

// Runner triages tickets in the background (HLD section 7).
type Runner interface {
	// Start begins a triage run for the ticket and returns at once.
	Start(ticketID string)
	// Wait blocks until in-flight runs finish, or returns ctx's error.
	Wait(ctx context.Context) error
}

// NopRunner does nothing. S1 wires it so the server runs before the real
// runner (S3) exists; tests that do not need triage can use it too.
type NopRunner struct{}

// Start does nothing.
func (NopRunner) Start(string) {}

// Wait returns at once.
func (NopRunner) Wait(context.Context) error { return nil }
