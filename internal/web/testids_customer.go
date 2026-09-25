package web

// data-testid values for the customer screen (HLD 3.1, 3.3). Owned by S4.
// Every name starts with "customer-" so it cannot collide with the agent
// screen's IDs.
const (
	TestIDCustomerForm        = "customer-form"
	TestIDCustomerName        = "customer-name"
	TestIDCustomerEmail       = "customer-email"
	TestIDCustomerTitle       = "customer-title"
	TestIDCustomerDescription = "customer-description"
	TestIDCustomerSubmit      = "customer-submit"
	TestIDCustomerErrors      = "customer-error-summary"
	// TestIDCustomerStatusPanel wraps the ticket's status; it is what the
	// status poll swaps.
	TestIDCustomerStatusPanel = "customer-status-panel"
	// TestIDCustomerTriaging is the "Working out where to send this…" text.
	TestIDCustomerTriaging       = "customer-triaging"
	TestIDCustomerFollowUpForm   = "customer-follow-up-form"
	TestIDCustomerFollowUpSubmit = "customer-follow-up-submit"
	// TestIDCustomerFollowUpAnswer is a prefix: each answer box's ID is
	// the prefix, "-", and the follow-up ID. Use FollowUpAnswerTestID.
	TestIDCustomerFollowUpAnswer = "customer-follow-up-answer"
)

// FollowUpAnswerTestID is the data-testid of the answer box for one
// follow-up question, e.g. "customer-follow-up-answer-location". The
// template builds the same string as {{tid_customer_follow_up_answer}}-{{.ID}}.
func FollowUpAnswerTestID(followUpID string) string {
	return TestIDCustomerFollowUpAnswer + "-" + followUpID
}

var _ = registerTestIDs(
	TestIDCustomerForm,
	TestIDCustomerName,
	TestIDCustomerEmail,
	TestIDCustomerTitle,
	TestIDCustomerDescription,
	TestIDCustomerSubmit,
	TestIDCustomerErrors,
	TestIDCustomerStatusPanel,
	TestIDCustomerTriaging,
	TestIDCustomerFollowUpForm,
	TestIDCustomerFollowUpSubmit,
	TestIDCustomerFollowUpAnswer,
)
