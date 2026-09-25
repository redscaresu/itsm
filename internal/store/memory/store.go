// Package memory is an in-memory domain.Store. Tickets are lost on restart.
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
)

// Store is a mutex-guarded map of tickets. It hands out and takes in deep
// copies, so no caller ever shares memory with what is stored.
type Store struct {
	mu      sync.Mutex
	tickets map[string]domain.Ticket
	next    int
	now     func() time.Time
}

var _ domain.Store = (*Store)(nil)

// New returns an empty store that stamps times with time.Now.
func New() *Store {
	return &Store{tickets: make(map[string]domain.Ticket), now: time.Now}
}

// Create assigns the next ID ("T-0001", "T-0002", …), sets CreatedAt and
// UpdatedAt, and saves a copy. Any ID the caller set is replaced.
func (s *Store) Create(_ context.Context, t *domain.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	now := s.now()
	t.ID = fmt.Sprintf("T-%04d", s.next)
	t.CreatedAt = now
	t.UpdatedAt = now
	s.tickets[t.ID] = t.Clone()
	return nil
}

// Get returns a copy of the ticket, or domain.ErrNotFound.
func (s *Store) Get(_ context.Context, id string) (domain.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return domain.Ticket{}, fmt.Errorf("get %q: %w", id, domain.ErrNotFound)
	}
	return t.Clone(), nil
}

// List returns copies of every ticket, oldest first.
func (s *Store) List(_ context.Context) ([]domain.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Ticket, 0, len(s.tickets))
	for _, t := range s.tickets {
		out = append(out, t.Clone())
	}
	// IDs are zero-padded and assigned in order, so they sort by age.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Update runs fn on a copy under the lock. It saves the copy and bumps
// UpdatedAt only if fn returns nil.
func (s *Store) Update(_ context.Context, id string, fn func(*domain.Ticket) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return fmt.Errorf("update %q: %w", id, domain.ErrNotFound)
	}
	c := t.Clone()
	if err := fn(&c); err != nil {
		return err
	}
	c.ID = id
	c.UpdatedAt = s.now()
	s.tickets[id] = c.Clone()
	return nil
}
