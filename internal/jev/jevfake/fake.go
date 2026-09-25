// Package jevfake is a fake jev server for tests (HLD 4.6). It answers per
// ticket, keyed on state.ticket.title, from a script the test supplies.
//
// It spells the wire format out itself rather than importing package jev,
// so a key typo in the client shows up as a failed decode, not a match.
package jevfake

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/redscaresu/itsm/internal/domain"
)

// Model is the model name the fake reports.
const Model = "jev-fake"

// Usage the fake reports on every answer.
const (
	InputTokens  = 1143
	OutputTokens = 198
)

// statusOverloaded is jev's "overloaded" status.
const statusOverloaded = 529

// Answers is one run's worth of answers. A zero confidence means 1.0; the
// probabilities put the confidence on the chosen option and share the rest
// evenly across the others.
type Answers struct {
	Team            domain.Team
	TeamConfidence  float64
	Kind            domain.Kind
	KindConfidence  float64
	Scope           domain.Scope
	ScopeConfidence float64
	Blocked         float64
	StatesLocation  float64
	StatesService   float64
}

// Default is what an unscripted title gets: a confident Service Desk
// question that affects one user and is missing nothing.
var Default = Answers{
	Team:           domain.TeamServiceDesk,
	Kind:           domain.KindQuestion,
	Scope:          domain.ScopeOneUser,
	StatesLocation: 1,
	StatesService:  1,
}

// Failure is how the fake answers a request instead of with Answers.
type Failure int

// Failures (HLD 4.6).
const (
	TooManyRequests Failure = iota + 1 // 429
	Overloaded                         // 529
	Hang                               // no answer until the client gives up
)

// Request is one request the fake received.
type Request struct {
	Authorization string
	Body          []byte
}

// Fake is a running fake jev. Point the client's base URL at Fake.URL.
type Fake struct {
	*httptest.Server

	mu       sync.Mutex
	script   map[string][]Answers
	runs     map[string]int
	failures []Failure
	requests []Request
	done     chan struct{}
}

// New starts a fake with an empty script. Close it when done.
func New() *Fake {
	f := &Fake{runs: map[string]int{}, done: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/systemone", f.handle)
	f.Server = httptest.NewServer(mux)
	return f
}

// Close releases any hanging request, then stops the server.
func (f *Fake) Close() {
	close(f.done)
	f.Server.Close()
}

// Script sets the answers per title: entry n answers run n+1 for that title,
// and the last entry repeats. It resets the run counts.
func (f *Fake) Script(s map[string][]Answers) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script = s
	f.runs = map[string]int{}
}

// FailNext makes the next len(fs) requests fail, in order.
func (f *Fake) FailNext(fs ...Failure) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, fs...)
}

// Requests returns every request received so far.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.requests...)
}

func (f *Fake) handle(w http.ResponseWriter, r *http.Request) {
	var in struct {
		State struct {
			Ticket struct {
				Title string `json:"title"`
			} `json:"ticket"`
		} `json:"state"`
	}
	body, err := io.ReadAll(r.Body)
	if err == nil {
		err = json.Unmarshal(body, &in)
	}
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	f.requests = append(f.requests, Request{Authorization: r.Header.Get("Authorization"), Body: body})
	var fail Failure
	if len(f.failures) > 0 {
		fail, f.failures = f.failures[0], f.failures[1:]
	}
	var a Answers
	if fail == 0 {
		a = f.next(in.State.Ticket.Title)
	}
	f.mu.Unlock()

	switch fail {
	case TooManyRequests:
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	case Overloaded:
		http.Error(w, "overloaded", statusOverloaded)
		return
	case Hang:
		select {
		case <-r.Context().Done():
		case <-f.done:
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(encode(a))
}

// next returns the answers for this title's next run. Call with f.mu held.
func (f *Fake) next(title string) Answers {
	runs, ok := f.script[title]
	if !ok || len(runs) == 0 {
		return Default
	}
	n := min(f.runs[title], len(runs)-1)
	f.runs[title]++
	return runs[n]
}

var (
	teamOptions  = []string{string(domain.TeamNetwork), string(domain.TeamEndUserCompute), string(domain.TeamApplications), string(domain.TeamIdentityAccess), string(domain.TeamServiceDesk)}
	kindOptions  = []string{string(domain.KindFault), string(domain.KindChange), string(domain.KindQuestion)}
	scopeOptions = []string{string(domain.ScopeOneUser), string(domain.ScopeOneOffice), string(domain.ScopeRegion), string(domain.ScopeNational), string(domain.ScopeUnclear)}
)

// encode builds the HLD 4.1 response body for one run.
func encode(a Answers) map[string]any {
	return map[string]any{
		"model": Model,
		"answers": map[string]any{
			"team":            choice(string(a.Team), a.TeamConfidence, teamOptions),
			"kind":            choice(string(a.Kind), a.KindConfidence, kindOptions),
			"scope":           choice(string(a.Scope), a.ScopeConfidence, scopeOptions),
			"blocked":         noul(a.Blocked),
			"states_location": noul(a.StatesLocation),
			"states_service":  noul(a.StatesService),
		},
		"usage": map[string]int{"input_tokens": InputTokens, "output_tokens": OutputTokens},
	}
}

func choice(option string, confidence float64, options []string) map[string]any {
	if confidence == 0 {
		confidence = 1
	}
	rest := (1 - confidence) / float64(len(options)-1)
	probs := make(map[string]float64, len(options))
	for _, o := range options {
		probs[o] = rest
	}
	probs[option] = confidence
	return map[string]any{"type": "choice", "choice": option, "confidence": confidence, "probabilities": probs}
}

func noul(p float64) map[string]any {
	return map[string]any{"type": "noul", "noul": p}
}
