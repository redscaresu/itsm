package jev_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/jev"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
)

// fastClient keeps retry tests quick; the defaults are 10 s and 500 ms.
func fastClient(url string) *jev.Client {
	return jev.NewClient(jev.Config{BaseURL: url, APIKey: "k", Timeout: 200 * time.Millisecond, Backoff: time.Millisecond})
}

func anyState() jev.State { return jev.NewState("title", "description", nil) }

// serveFile answers every request with the file's bytes and the given status.
func serveFile(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Catches: a JSON key typo on the response side (e.g. "input_token",
// "probability") decoding to zero, or an answer wired to the wrong field.
func TestJudgeDecodesLiveResponse(t *testing.T) {
	body, err := os.ReadFile("testdata/live_response.json")
	require.NoError(t, err)
	srv := serveFile(t, http.StatusOK, body)

	j, err := fastClient(srv.URL).Judge(context.Background(), anyState())
	require.NoError(t, err)

	assert.Equal(t, "jev-1.13.0", j.Model)
	assert.Equal(t, domain.Usage{InputTokens: 1143, OutputTokens: 198}, j.Usage)
	assert.Positive(t, j.Latency)
	assert.Equal(t, domain.Choice{Option: "network", Confidence: 0.98, Probabilities: map[string]float64{
		"network": 0.98, "end_user_compute": 0.01, "applications": 0, "identity_access": 0, "service_desk": 0.01,
	}}, j.Team)
	assert.Equal(t, domain.Choice{Option: "fault", Confidence: 0.99, Probabilities: map[string]float64{
		"fault": 0.99, "change": 0.01, "question": 0,
	}}, j.Kind)
	assert.Equal(t, domain.Choice{Option: "one_office", Confidence: 0.97, Probabilities: map[string]float64{
		"one_user": 0, "one_office": 0.97, "region": 0.02, "national": 0, "unclear": 0.01,
	}}, j.Scope)
	assert.InDelta(t, 0.95, j.Blocked, 1e-9)
	assert.InDelta(t, 0.93, j.StatesLocation, 1e-9)
	assert.InDelta(t, 0.81, j.StatesService, 1e-9)
	// The rules (S3) and the triager own these; the client must leave them zero.
	assert.Zero(t, j.Severity)
	assert.Empty(t, j.Rule)
	assert.False(t, j.NeedsReview)
	assert.Empty(t, j.Error)
	assert.True(t, j.CompletedAt.IsZero())
}

// Catches: a missing or mistyped answer decoding to zero and routing the
// ticket on made-up values instead of failing the run.
func TestJudgeRejectsMissingAnswer(t *testing.T) {
	body, err := os.ReadFile("testdata/live_response.json")
	require.NoError(t, err)
	broken := strings.Replace(string(body), `"states_service"`, `"states_svc"`, 1)
	srv := serveFile(t, http.StatusOK, []byte(broken))

	_, err = fastClient(srv.URL).Judge(context.Background(), anyState())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "states_service")
}

// Catches: the retry being dropped, or narrowed so that 429 or 529 no
// longer gets a second attempt.
func TestJudgeRetriesOnceOn429And529(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail jevfake.Failure
	}{
		{"429", jevfake.TooManyRequests},
		{"529", jevfake.Overloaded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := jevfake.New()
			defer fake.Close()
			fake.FailNext(tc.fail)

			j, err := fastClient(fake.URL).Judge(context.Background(), anyState())
			require.NoError(t, err)
			assert.Equal(t, jevfake.Model, j.Model)
			assert.Len(t, fake.Requests(), 2)
		})
	}
}

// Catches: a retry loop that keeps going, so a jev outage holds a ticket
// in triaging for longer than the HLD's ~20 s worst case.
func TestJudgeGivesUpAfterOneRetry(t *testing.T) {
	fake := jevfake.New()
	defer fake.Close()
	fake.FailNext(jevfake.TooManyRequests, jevfake.Overloaded, jevfake.TooManyRequests)

	_, err := fastClient(fake.URL).Judge(context.Background(), anyState())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "529")
	assert.Len(t, fake.Requests(), 2)
}

// Catches: the per-call timeout being lost (a hung jev would hang the run
// forever) or a timeout not being retried.
func TestJudgeTimeoutIsAnErrorAfterOneRetry(t *testing.T) {
	fake := jevfake.New()
	defer fake.Close()
	fake.FailNext(jevfake.Hang, jevfake.Hang)

	_, err := fastClient(fake.URL).Judge(context.Background(), anyState())
	require.ErrorIs(t, err, jev.ErrTimeout)
	assert.Len(t, fake.Requests(), 2)
}

// Catches: retrying a request jev rejected as bad, which only doubles the
// wait before the ticket falls back to Service Desk.
func TestJudgeDoesNotRetryOtherStatuses(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, "bad question", http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := fastClient(srv.URL).Judge(context.Background(), anyState())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Equal(t, 1, calls)
}
