package demodata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/triage"
)

var demoFile = filepath.Join("..", "..", File)

// Regression: if tickets.json is edited so its scripted jev answers no
// longer give its own expected team, severity and status under the section 5
// rules, the fake-driven tests and the walkthrough fail far from the cause.
// This names the ticket and run instead.
func TestScriptedAnswersGiveExpectedOutcome(t *testing.T) {
	tickets, err := Load(demoFile)
	require.NoError(t, err)
	require.Len(t, tickets, 6, "HLD section 10 has six demo tickets")

	for _, tk := range tickets {
		first := triage.Decide(judgment(tk.Jev[0]), true)
		assert.Equal(t, tk.Expect.Team, first.Team, "ticket %d team", tk.N)
		assert.Equal(t, tk.Expect.Severity, first.Severity, "ticket %d severity", tk.N)
		assert.Equal(t, tk.Expect.Status, first.Status, "ticket %d status", tk.N)
		assert.Equal(t, tk.Expect.NeedsReview, first.NeedsReview, "ticket %d needs review", tk.N)
		if tk.ExpectAfter == nil {
			continue
		}
		// Every follow-up the first run asks must have a scripted answer.
		for _, f := range first.FollowUps {
			assert.NotEmpty(t, tk.Answers.For(f.ID), "ticket %d has no answer for %q", tk.N, f.ID)
		}
		second := triage.Decide(judgment(tk.Jev[1]), false)
		assert.Equal(t, tk.ExpectAfter.Team, second.Team, "ticket %d team after", tk.N)
		assert.Equal(t, tk.ExpectAfter.Severity, second.Severity, "ticket %d severity after", tk.N)
		assert.Equal(t, domain.StatusOpen, second.Status, "ticket %d status after", tk.N)
	}
}

// Regression: if Load stops rejecting unknown keys, a misspelt key such as
// "expect_afer" decodes to nil and ticket 4's second run goes unchecked.
func TestLoadRejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tickets.json")
	writeFile(t, path, `[{"n":1,"title":"t","description":"d","jev":[{}],"expect_afer":{}}]`)
	_, err := Load(path)
	require.ErrorContains(t, err, "expect_afer")
}

func judgment(r Run) domain.Judgment {
	return domain.Judgment{
		Team:           domain.Choice{Option: string(r.Team), Confidence: r.TeamConfidence},
		Kind:           domain.Choice{Option: string(r.Kind), Confidence: 1},
		Scope:          domain.Choice{Option: string(r.Scope), Confidence: r.ScopeConfidence},
		Blocked:        r.Blocked,
		StatesLocation: r.StatesLocation,
		StatesService:  r.StatesService,
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}
