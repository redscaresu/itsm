package web

// data-testid values for the agent screen (HLD 3.2, 3.3). Owned by S5.
// Each queue row and feed line also carries data-ticket="T-0001", so the
// walkthrough can pick one ticket's row by ID, not by visible text.
const (
	TestIDAgentQueue          = "agent-queue"
	TestIDAgentQueueRow       = "agent-queue-row"
	TestIDAgentTicketLink     = "agent-ticket-link"
	TestIDAgentFilterTeam     = "agent-filter-team"
	TestIDAgentFilterSeverity = "agent-filter-severity"
	TestIDAgentFilterSubmit   = "agent-filter-submit"
	TestIDAgentFeed           = "agent-feed"
	TestIDAgentFeedLine       = "agent-feed-line"
	TestIDAgentWhyPanel       = "agent-why-panel"
	TestIDAgentWhyRun         = "agent-why-run"
	TestIDAgentWhyRule        = "agent-why-rule"
	TestIDAgentReassignTeam   = "agent-reassign-team"
	TestIDAgentReassignSubmit = "agent-reassign-submit"
	TestIDAgentSeveritySelect = "agent-severity-select"
	TestIDAgentSeveritySubmit = "agent-severity-submit"
	TestIDAgentCloseSubmit    = "agent-close-submit"
	TestIDAgentOverride       = "agent-override"
)

var _ = registerTestIDs(
	TestIDAgentQueue,
	TestIDAgentQueueRow,
	TestIDAgentTicketLink,
	TestIDAgentFilterTeam,
	TestIDAgentFilterSeverity,
	TestIDAgentFilterSubmit,
	TestIDAgentFeed,
	TestIDAgentFeedLine,
	TestIDAgentWhyPanel,
	TestIDAgentWhyRun,
	TestIDAgentWhyRule,
	TestIDAgentReassignTeam,
	TestIDAgentReassignSubmit,
	TestIDAgentSeveritySelect,
	TestIDAgentSeveritySubmit,
	TestIDAgentCloseSubmit,
	TestIDAgentOverride,
)
