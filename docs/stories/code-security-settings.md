---
kind: lead
repo: github-actions
status: blocked
blocked_by: [gate-zizmor]
epic: bundle-gates
depends_on: [gate-zizmor]
touches: []
---

# Owner turns on Dependabot alerts and private vulnerability reporting; lead confirms the dependency graph answers

Needs your go-ahead and a Settings session on github-actions, after gate-zizmor merges (go.mod on main). **You:** Settings → Code security: turn on Dependabot alerts and private vulnerability reporting. Leave Dependabot security updates off until ruleset-and-pinning, so no bump PR opens before the ruleset (critic, review). Leave code scanning default setup unclicked. Tell the lead when done. Up front: GitHub likely offers no dependency-graph toggle on a public repo, so today's 403 (survey fact 9) more likely means no manifest yet or alerts off; the lead re-probes now that go.mod is on main. Still 403: stop, the lead adds a lead story, gate-dependency-review stays blocked.

**Done when:** `gh api repos/redscaresu/github-actions --jq .security_and_analysis` shows `secret_scanning.status` and `secret_scanning_push_protection.status` `enabled` (still on) and `dependabot_security_updates.status` `disabled` (turned on later); `gh api repos/redscaresu/github-actions/vulnerability-alerts` returns 204; `gh api repos/redscaresu/github-actions/private-vulnerability-reporting --jq .enabled` is true; `gh api repos/redscaresu/github-actions/dependency-graph/compare/<parent>...<gate-zizmor merge SHA>` returns 200, recorded here with both SHAs before gate-dependency-review is marked ready; `gh api repos/redscaresu/github-actions/code-scanning/default-setup --jq .state` is `not-configured`.
