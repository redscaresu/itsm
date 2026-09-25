package main

import (
	"context"
	"io"
	"io/fs"
	"net"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/web"
)

// Smoke test only. S6 replaces this file with the full lifecycle test.

// Regression: if the key check is dropped, the server starts without a key
// and every triage run fails at the first jev call instead of at startup.
func TestRunFailsWithoutAPIKey(t *testing.T) {
	// Bounded, so a server that wrongly starts returns nil instead of hanging.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err := run(ctx, io.Discard, nil, env(map[string]string{"PORT": freePort(t)}))
	require.ErrorContains(t, err, "TYPESAFE_API_KEY")
}

// Regression: if /healthz, the /static/ mount or the /assets/ mount breaks,
// the demo loses its readiness check, htmx, or the GOV.UK fonts; and if
// shutdown stops returning cleanly, run() hangs or errors on Ctrl-C.
func TestRunServesHealthzAndAssets(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, io.Discard, nil, env(map[string]string{"TYPESAFE_API_KEY": "x", "PORT": port}))
	}()
	base := "http://127.0.0.1:" + port
	waitForReady(t, base+"/healthz")

	assertOK(t, base+"/static/htmx.min.js")

	css, err := fs.Glob(web.Static(), "govuk/govuk-frontend-*.min.css")
	require.NoError(t, err)
	require.Len(t, css, 1)
	body := assertOK(t, base+"/static/"+css[0])
	font := regexp.MustCompile(`/assets/fonts/[^)"']+`).FindString(body)
	require.NotEmpty(t, font, "govuk CSS names no font")
	assertOK(t, base+font)

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// freePort asks the OS for a free port. Another process could take it
// before run() binds; that is rare enough for a test.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	require.NoError(t, l.Close())
	return port
}

// waitForReady polls url until it answers 200 or the test times out.
func waitForReady(t *testing.T, url string) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)
}

func assertOK(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, url)
	return string(b)
}
