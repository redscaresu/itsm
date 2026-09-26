package jev

import (
	"context"
	"time"

	"github.com/redscaresu/itsm/internal/domain"
)

// Triager implements domain.Triager over a Client.
type Triager struct {
	client *Client
	now    func() time.Time
}

var _ domain.Triager = (*Triager)(nil)

// NewTriager returns a Triager that asks jev through client.
func NewTriager(client *Client) *Triager {
	return &Triager{client: client, now: time.Now}
}

// Triage sends the ticket's title, description and answered follow-ups to
// jev and returns its answers with CompletedAt set. Severity, Rule and
// NeedsReview are left zero: the rules (S3) fill them in.
func (t *Triager) Triage(ctx context.Context, tk domain.Ticket) (domain.Judgment, error) {
	var answers []FollowUpAnswer
	for _, f := range tk.FollowUps {
		if f.Answer == "" {
			continue
		}
		answers = append(answers, FollowUpAnswer{Question: f.Question, Answer: f.Answer})
	}
	j, err := t.client.Judge(ctx, NewState(tk.Title, tk.Description, answers))
	if err != nil {
		return domain.Judgment{}, err
	}
	j.CompletedAt = t.now()
	return j, nil
}
