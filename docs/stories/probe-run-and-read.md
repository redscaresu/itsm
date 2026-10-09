---
kind: lead
repo: github-actions
status: blocked
blocked_by: [probe-workflows]
epic: bundle-scorecard-probe
depends_on: [probe-workflows]
touches: [docs/stories/probe-answer-and-removal.md]
risk: high
---

# Merge the probe and read the Scorecard result with the owner

Lead, before merge: online `zizmor .github/` (GH_TOKEN, default persona) on the PR head reports 0; `curl -s https://api.github.com/repos/redscaresu/github-actions/tags` is `[]`. **You:** merge the probe PR to main (no protection yet; it grants id-token: write and publishes to a public API; the lead sees the merge). Lead: find the push run of probe-scorecard.yml for the merge SHA; if the called job is not green, stop and add a fix story (no verdict from a red run). Within an hour curl /projects/github.com/redscaresu/github-actions on api.securityscorecards.dev and api.scorecard.dev, and read the scorecard-action step log. REFUTED if `repo.commit` == merge SHA; VERIFIED if not and the log carries the publish refusal/warning; neither is inconclusive: rerun once, then **You:** decide. **You:** read the result (API JSON or log line) with the lead. If REFUTED, **You:** confirm decision 6 (HLD:409, Scorecard becomes bundle job `scorecard`) or choose the template anyway; the lead learns it from your reply. Lead writes verdict, run URL, evidence and your choice into probe-answer-and-removal and probe-unblock-epics.

**Done when:** Online `zizmor .github/` reported 0 findings on the PR head before merge; a push run on main for the merge SHA has job `scorecard / Scorecard analysis` with conclusion success; docs/stories/probe-answer-and-removal.md holds exactly VERIFIED or REFUTED (never inconclusive), the run URL, and as evidence the API's `repo.commit` or the log's refusal line, consistent with the rule (REFUTED iff repo.commit == merge SHA); when REFUTED, the owner's choice (decision 6 or template) is recorded; tags still `[]`. Evidence rules: REFUTED quotes `repo.commit` equal to the merge SHA from at least one of api.securityscorecards.dev and api.scorecard.dev. VERIFIED needs all three: the merge SHA, a curl of both hosts showing 404 or a different repo.commit, and the quoted scorecard-action publish refusal/warning line from the step log. Still inconclusive after one rerun: record no verdict, leave probe-answer-and-removal blocked, and add a new lead story for the owner's choice (rerun later, or treat as REFUTED). The lead commits what it records to itsm main through a PR.
