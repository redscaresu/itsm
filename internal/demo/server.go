package demo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// Timings for starting and stopping the itsm server.
const (
	readyTimeout = 15 * time.Second
	readyPoll    = 100 * time.Millisecond
	// stopTimeout covers the server's own 10 s drain (HLD 7) plus margin.
	stopTimeout = 15 * time.Second
)

// Server is an itsm binary running as a child process.
type Server struct {
	cmd  *exec.Cmd
	done chan error
}

// StartServer runs the itsm binary at bin with the given extra environment
// (on top of this process's), and waits until baseURL/healthz answers 200.
func StartServer(ctx context.Context, bin, baseURL string, env []string, out io.Writer) (*Server, error) {
	// No context: cancelling one would SIGKILL the server. Stop sends SIGINT
	// so it drains its triage runs first.
	cmd := exec.Command(bin) //nolint:noctx // see above
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("demo: start %s: %w", bin, err)
	}
	s := &Server{cmd: cmd, done: make(chan error, 1)}
	go func() { s.done <- cmd.Wait() }()
	if err := s.waitReady(ctx, baseURL+"/healthz"); err != nil {
		_ = s.Stop()
		return nil, err
	}
	return s, nil
}

// waitReady polls url until it answers 200, the server exits, or time runs out.
func (s *Server) waitReady(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	tick := time.NewTicker(readyPoll)
	defer tick.Stop()
	for {
		if healthy(ctx, url) {
			return nil
		}
		select {
		case err := <-s.done:
			s.done <- err // Stop still needs it.
			if err == nil {
				err = errors.New("exit status 0")
			}
			return fmt.Errorf("demo: server exited before it was ready: %w", err)
		case <-ctx.Done():
			return fmt.Errorf("demo: %s not ready: %w", url, ctx.Err())
		case <-tick.C:
		}
	}
}

func healthy(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Stop sends SIGINT, so the server drains cleanly, and kills it if it has
// not exited within stopTimeout.
func (s *Server) Stop() error {
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("demo: stop server: %w", err)
	}
	select {
	case err := <-s.done:
		return err
	case <-time.After(stopTimeout):
		_ = s.cmd.Process.Kill()
		return fmt.Errorf("demo: server did not stop within %s; killed", stopTimeout)
	}
}
