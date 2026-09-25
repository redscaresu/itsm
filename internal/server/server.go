// Package server is the HTTP layer: NewServer, routes, middleware and the
// render helpers. It follows Mat Ryer's "How I write HTTP services in Go
// after 13 years" (HLD section 7).
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/web"
)

// Config is read from the environment by run() (HLD section 8).
type Config struct {
	APIKey  string // TYPESAFE_API_KEY; never logged
	BaseURL string // TYPESAFE_BASE_URL
	Port    string // PORT
}

// deps is everything a handler may use. It is frozen: wave-B stories build
// handlers against these five fields and do not add more.
type deps struct {
	store     domain.Store
	runner    domain.Runner
	templates *web.Templates
	clock     func() time.Time
	logger    *slog.Logger
}

// NewServer builds the mux and applies middleware. The server never sees
// the Triager; only the runner does. cfg is accepted to match the HLD
// signature; no handler reads it yet.
func NewServer(logger *slog.Logger, _ Config, store domain.Store, runner domain.Runner) http.Handler {
	d := deps{
		store:     store,
		runner:    runner,
		templates: web.MustParse(),
		clock:     time.Now,
		logger:    logger,
	}
	mux := http.NewServeMux()
	addRoutes(mux, d)
	// recover sits inside logging, so a panicking request is still logged,
	// with the 500 that recover wrote.
	var h http.Handler = mux
	h = recoverPanics(logger, h)
	h = logRequests(logger, h)
	return h
}
