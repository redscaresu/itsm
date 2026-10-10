---
kind: lead
repo: github-actions
status: blocked
blocked_by: [gate-dependency-review]
epic: bundle-gates
depends_on: [gate-dependency-review]
touches: []
risk: high
---

# Owner adds the main ruleset (ten checks, CodeQL results, no bypass), then Dependabot security updates; optional SHA-pinning policy

Needs your go-ahead and a Settings session, after gate-dependency-review merges (its PR run is the first with all five gates, contradiction 5). **You:** create a ruleset on `main`, enforcement active, bypass list empty: Require status checks, source GitHub Actions, the names that run shows: `security / zizmor`, `security / gitleaks`, `security / govulncheck`, `security / codeql`, `security / dependency-review` and the same five under `security-codeql-off /`; not `security / scorecard` (push-only, decision 2). Also Require code scanning results, tool CodeQL at the defaults (errors; high or higher), because `security / codeql` succeeds even with an alert (gate-codeql red proof); if Settings does not offer it, require check `CodeQL` instead and the lead records which. **You:** then turn on Dependabot security updates (moved here so every bump goes through the ruleset). From then on every PR here, every Dependabot bump included, goes through the ruleset and you read the diff before merging (HLD:280-282, 328-329). Optional **You:** Actions → General → 'Require actions to be pinned to a full-length commit SHA'; if on, the lead re-runs the last PR's checks, and any red means you turn it off and the lead records why.

**Done when:** `gh api repos/redscaresu/github-actions/rulesets` lists one ruleset; `gh api repos/redscaresu/github-actions/rulesets/<id>` shows enforcement `active`, `bypass_actors: []`, `conditions.ref_name.include` containing `refs/heads/main` or `~DEFAULT_BRANCH`, a required_status_checks rule whose contexts are exactly the ten names, each with `integration_id` 15368, no `scorecard`, and a `code_scanning` rule with tool `CodeQL` (or the recorded `CodeQL` check fallback). `gh api repos/redscaresu/github-actions --jq .security_and_analysis.dependabot_security_updates.status` is `enabled`. `gh api repos/redscaresu/github-actions/actions/permissions --jq .sha_pinning_required` is recorded: true with a green re-run, or false with the reason.
