package jev_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/redscaresu/itsm/internal/domain"
	"github.com/redscaresu/itsm/internal/jev"
	"github.com/redscaresu/itsm/internal/jev/jevfake"
)

const vagueTitle = "Everything is broken"

// Catches: the fake answering every run of a ticket the same way, which
// would make ticket 4's re-route untestable in S6 and S7.
func TestFakeAnswersEachRunFromTheScript(t *testing.T) {
	fake := jevfake.New()
	defer fake.Close()
	fake.Script(map[string][]jevfake.Answers{vagueTitle: {
		{Team: domain.TeamServiceDesk, Kind: domain.KindFault, Scope: domain.ScopeUnclear},
		{Team: domain.TeamApplications, TeamConfidence: 0.96, Kind: domain.KindFault, Scope: domain.ScopeOneUser},
	}})
	tr := jev.NewTriager(fastClient(fake.URL))
	tk := domain.Ticket{Title: vagueTitle, Description: "Everything is broken, please help."}

	first, err := tr.Triage(context.Background(), tk)
	require.NoError(t, err)
	second, err := tr.Triage(context.Background(), tk)
	require.NoError(t, err)
	other, err := tr.Triage(context.Background(), domain.Ticket{Title: "not in the script"})
	require.NoError(t, err)

	assert.Equal(t, string(domain.TeamServiceDesk), first.Team.Option)
	assert.Equal(t, string(domain.ScopeUnclear), first.Scope.Option)
	assert.Equal(t, string(domain.TeamApplications), second.Team.Option)
	assert.InDelta(t, 0.96, second.Team.Confidence, 1e-9)
	assert.Equal(t, string(domain.ScopeOneUser), second.Scope.Option)
	assert.Equal(t, string(jevfake.Default.Team), other.Team.Option)
}

// Catches: the second run not sending the customer's answers (jev would
// see the same vague ticket and route it the same way), or sending the
// unanswered ones as empty strings.
func TestTriageSendsAnsweredFollowUps(t *testing.T) {
	fake := jevfake.New()
	defer fake.Close()
	tr := jev.NewTriager(fastClient(fake.URL))
	tk := domain.Ticket{Title: vagueTitle, FollowUps: []domain.FollowUp{
		{ID: domain.FollowUpLocation, Question: "Which office or site is affected? If it's everywhere, or just you, say so.", Answer: "Bristol office"},
		{ID: domain.FollowUpService, Question: "Which system, application or device has the problem?", Answer: ""},
		{ID: domain.FollowUpImpact, Question: "Roughly how many people are affected — just you, your office, or more?", Answer: "just me"},
	}}

	before := time.Now()
	j, err := tr.Triage(context.Background(), tk)
	require.NoError(t, err)
	assert.False(t, j.CompletedAt.Before(before), "CompletedAt drives the feed order")

	reqs := fake.Requests()
	require.Len(t, reqs, 1)
	var sent struct {
		State struct {
			Ticket struct {
				FollowUpAnswers json.RawMessage `json:"follow_up_answers"`
			} `json:"ticket"`
		} `json:"state"`
	}
	require.NoError(t, json.Unmarshal(reqs[0].Body, &sent))
	assert.JSONEq(t, `[
		{"question": "Which office or site is affected? If it's everywhere, or just you, say so.", "answer": "Bristol office"},
		{"question": "Roughly how many people are affected — just you, your office, or more?", "answer": "just me"}
	]`, string(sent.State.Ticket.FollowUpAnswers))
}

// Catches: a jev failure being swallowed into a zero Judgment, which the
// runner would then route as if jev had answered.
func TestTriageReturnsClientError(t *testing.T) {
	fake := jevfake.New()
	defer fake.Close()
	fake.FailNext(jevfake.Hang, jevfake.Hang)

	_, err := jev.NewTriager(fastClient(fake.URL)).Triage(context.Background(), domain.Ticket{Title: "x"})
	require.ErrorIs(t, err, jev.ErrTimeout)
}
