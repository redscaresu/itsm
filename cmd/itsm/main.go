// Command itsm is the IT self-service portal server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redscaresu/itsm/internal/jev"
	"github.com/redscaresu/itsm/internal/server"
	"github.com/redscaresu/itsm/internal/store/memory"
	"github.com/redscaresu/itsm/internal/triage"
)

const (
	defaultBaseURL = "https://api.typesafe.ai"
	defaultPort    = "8080"
	// drainTimeout bounds the whole shutdown: HTTP drain plus runner.Wait.
	drainTimeout      = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
)

func main() {
	if err := run(context.Background(), os.Stdout, os.Args, os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "itsm: %s\n", err)
		os.Exit(1)
	}
}

// run is the whole program. It returns when ctx is cancelled or SIGINT /
// SIGTERM arrives, after a graceful shutdown (HLD section 7).
func run(ctx context.Context, w io.Writer, _ []string, getenv func(string) string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(w, nil))

	// Triage runs use their own base context: it outlives each request and
	// the shutdown signal, and is cancelled only when run() returns, which is
	// after runner.Wait or once the drain expires (HLD section 7).
	baseCtx, cancelRuns := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRuns()
	store := memory.New()
	triager := jev.NewTriager(jev.NewClient(jev.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey}))
	runner := triage.NewRunner(baseCtx, triage.Deps{Store: store, Triager: triager, Logger: logger})

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           server.NewServer(logger, cfg, store, runner),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainTimeout)
	defer cancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := runner.Wait(drainCtx); err != nil {
		return fmt.Errorf("wait for triage runs: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// loadConfig reads the environment (HLD section 8). The API key is required
// and is never logged or echoed.
func loadConfig(getenv func(string) string) (server.Config, error) {
	cfg := server.Config{
		APIKey:  getenv("TYPESAFE_API_KEY"),
		BaseURL: getenv("TYPESAFE_BASE_URL"),
		Port:    getenv("PORT"),
	}
	if cfg.APIKey == "" {
		return cfg, errors.New("TYPESAFE_API_KEY is not set")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	return cfg, nil
}
