package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/web"
)

type testForm struct {
	Title    string `form:"title"`
	Severity int    `form:"severity"`
	Ignored  string
}

func (f testForm) Valid(context.Context) map[string]string {
	if f.Title == "" {
		return map[string]string{"title": "Enter a title"}
	}
	return nil
}

func postForm(values url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// Regression: if the form-tag mapping breaks, every submitted field decodes
// empty and every customer and agent form fails validation.
func TestDecodeFormFillsTaggedFields(t *testing.T) {
	got, problems, err := decodeForm[testForm](postForm(url.Values{
		"title": {"Leeds office network down"}, "severity": {"2"}, "Ignored": {"x"},
	}))
	require.NoError(t, err)
	assert.Empty(t, problems)
	assert.Equal(t, testForm{Title: "Leeds office network down", Severity: 2}, got)
}

// Regression: an invalid form must come back as problems (re-render the
// form), not as an error (400) and not as a silent success.
func TestDecodeFormReturnsProblems(t *testing.T) {
	_, problems, err := decodeForm[testForm](postForm(url.Values{"title": {""}}))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"title": "Enter a title"}, problems)
}

// Regression: a non-numeric severity must be a 400, not silently Sev 0.
func TestDecodeFormRejectsBadInt(t *testing.T) {
	_, _, err := decodeForm[testForm](postForm(url.Values{"title": {"x"}, "severity": {"two"}}))
	require.Error(t, err)
}

// Regression: the flash is decided server-side from since (HLD 3.2). If a
// missing since meant "the zero time", every row would flash on first load;
// if the round trip lost precision, a row changed in the same second as the
// last poll would never flash.
func TestSinceFlash(t *testing.T) {
	now := time.Date(2026, 9, 25, 9, 0, 0, 123456789, time.UTC)
	d := deps{clock: func() time.Time { return now }}

	missing, err := d.parseSince(httptest.NewRequest(http.MethodGet, "/agent/queue", nil))
	require.NoError(t, err)
	assert.Empty(t, flashClass(now.Add(-time.Second), missing), "first load flashes nothing")

	req := httptest.NewRequest(http.MethodGet, "/agent/queue?"+sinceParam+"="+sinceValue(now), nil)
	since, err := d.parseSince(req)
	require.NoError(t, err)
	assert.Equal(t, web.FlashClass, flashClass(now.Add(time.Nanosecond), since), "changed after the last poll")
	assert.Empty(t, flashClass(now, since), "unchanged since the last poll")

	_, err = d.parseSince(httptest.NewRequest(http.MethodGet, "/agent/queue?since=yesterday", nil))
	require.Error(t, err)
}

// Regression: if render writes the status before the template has executed,
// a template error goes out as a 200 with a broken page instead of a 500.
func TestRenderErrorAnswers500(t *testing.T) {
	d := deps{templates: web.MustParse(), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	d.render(rec, http.StatusOK, "customer/no-such-page.html", nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
