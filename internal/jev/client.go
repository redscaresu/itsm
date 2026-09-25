// Package jev is the only code that talks to jev, TypeSafe's System One
// model (HLD section 4).
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
)

// Model is the jev model the request asks for (HLD decision 2).
const Model = "jev-latest"

// Endpoint is the path of the System One API on the base URL.
const Endpoint = "/v1/systemone"

// Defaults (HLD 4.6).
const (
	DefaultTimeout = 10 * time.Second
	DefaultBackoff = 500 * time.Millisecond
)

// statusOverloaded is jev's "overloaded" status; net/http has no name for it.
const statusOverloaded = 529

// maxResponseBytes caps how much of a response body is read.
const maxResponseBytes = 1 << 20

// maxErrorBody caps how much of an error body goes into an error message.
const maxErrorBody = 200

// ErrTimeout is wrapped into the error when a call timed out on both tries.
var ErrTimeout = errors.New("timeout")

// Config configures a Client. Zero Timeout and Backoff take the defaults.
type Config struct {
	BaseURL string // e.g. https://api.typesafe.ai, no trailing path
	APIKey  string // sent as a bearer token; never logged
	Timeout time.Duration
	Backoff time.Duration
}

// Client calls POST {BaseURL}/v1/systemone.
type Client struct {
	url     string
	apiKey  string
	backoff time.Duration
	http    *http.Client
}

// NewClient returns a Client for cfg.
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Backoff == 0 {
		cfg.Backoff = DefaultBackoff
	}
	return &Client{
		url:     strings.TrimRight(cfg.BaseURL, "/") + Endpoint,
		apiKey:  cfg.APIKey,
		backoff: cfg.Backoff,
		http:    &http.Client{Timeout: cfg.Timeout},
	}
}

// request is the body sent to jev (HLD 4.1).
type request struct {
	Model     string              `json:"model"`
	State     State               `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Judge asks the HLD 4.2 questions about state and returns jev's answers as a
// Judgment with Model, Latency, Usage and the six answers filled in. Severity,
// Rule, NeedsReview and CompletedAt are left for the caller. Latency is the
// wall-clock time of the whole call, including a retry.
func (c *Client) Judge(ctx context.Context, state State) (domain.Judgment, error) {
	body, err := json.Marshal(request{Model: Model, State: state, Questions: questions()})
	if err != nil {
		return domain.Judgment{}, fmt.Errorf("jev: encode request: %w", err)
	}
	start := time.Now()
	raw, err := c.post(ctx, body)
	if err != nil && retryable(ctx, err) {
		if werr := wait(ctx, c.backoff); werr != nil {
			return domain.Judgment{}, fmt.Errorf("jev: %w", werr)
		}
		raw, err = c.post(ctx, body)
	}
	if err != nil {
		if isTimeout(err) {
			return domain.Judgment{}, fmt.Errorf("jev: %w: %w", ErrTimeout, err)
		}
		return domain.Judgment{}, fmt.Errorf("jev: %w", err)
	}
	j, err := decode(raw)
	if err != nil {
		return domain.Judgment{}, fmt.Errorf("jev: %w", err)
	}
	j.Latency = time.Since(start)
	return j, nil
}

// statusError is a non-2xx answer from jev.
type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("status %d: %s", e.code, e.body)
}

// post makes one attempt and returns the response body on a 2xx.
func (c *Client) post(ctx context.Context, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := string(raw)
		if len(msg) > maxErrorBody {
			msg = msg[:maxErrorBody]
		}
		return nil, &statusError{code: resp.StatusCode, body: msg}
	}
	return raw, nil
}

// retryable reports whether one more attempt is worth making: a 429, a 529,
// or a network error including a client timeout. Nothing is retried once the
// caller's own context is done.
func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var se *statusError
	if errors.As(err, &se) {
		return se.code == http.StatusTooManyRequests || se.code == statusOverloaded
	}
	var ne net.Error
	return errors.As(err, &ne)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// response is jev's reply (HLD 4.1). Pointers tell "missing" from zero, so a
// key typo on either side is an error rather than a silent zero.
type response struct {
	Model   string            `json:"model"`
	Answers map[string]answer `json:"answers"`
	Usage   *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Noul          *float64           `json:"noul"`
}

// decode turns a response body into a Judgment, checking that every question
// in the set came back with the type it was asked as.
func decode(raw []byte) (domain.Judgment, error) {
	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return domain.Judgment{}, fmt.Errorf("decode response: %w", err)
	}
	if r.Model == "" {
		return domain.Judgment{}, errors.New("decode response: no model")
	}
	if r.Usage == nil {
		return domain.Judgment{}, errors.New("decode response: no usage")
	}
	j := domain.Judgment{
		Model: r.Model,
		Usage: domain.Usage{InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens},
	}
	choices := map[string]*domain.Choice{QuestionTeam: &j.Team, QuestionKind: &j.Kind, QuestionScope: &j.Scope}
	for id, dst := range choices {
		a, ok := r.Answers[id]
		if !ok || a.Type != TypeChoice || a.Choice == "" || a.Confidence == nil {
			return domain.Judgment{}, fmt.Errorf("decode response: answer %q missing or not a choice", id)
		}
		*dst = domain.Choice{Option: a.Choice, Confidence: *a.Confidence, Probabilities: a.Probabilities}
	}
	nouls := map[string]*float64{QuestionBlocked: &j.Blocked, QuestionStatesLocation: &j.StatesLocation, QuestionStatesService: &j.StatesService}
	for id, dst := range nouls {
		a, ok := r.Answers[id]
		if !ok || a.Type != TypeNoul || a.Noul == nil {
			return domain.Judgment{}, fmt.Errorf("decode response: answer %q missing or not a noul", id)
		}
		*dst = *a.Noul
	}
	return j, nil
}

// wait sleeps for d or until ctx is done.
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
