---
status: active
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
---

# Answer HLD:143-147 before anything else lands in redscaresu/github-actions: can OpenSSF Scorecard publish a score from a job that runs inside an `on: workflow_call` workflow? One reusable `scorecard.yml` plus a push:main caller, merged, run once, read, then removed. The answer decides whether Scorecard is a template (expected) or a bundle job (decision 6); bundle-gates, bundle-templates-readme and public-only-gates stay `later` until it is in.

**Done when:** In redscaresu/github-actions: a push run on `main` of caller `.github/workflows/probe-scorecard.yml` (one job, `uses: $/.github/workflows/scorecard.yml`, the form that ran green; zizmor flags `./`, `permissions: {contents: read, id-token: write}`) completed with the called job green, and neither file has workflow-level `env:` or `defaults:` (Scorecard refuses to publish from one and only warns, goldfinger scorecard.yml:19-24, which would fake a negative). The answer is recorded in README.md under `## Scorecard` with the run URL: REFUTED if `curl -s https://api.securityscorecards.dev/projects/github.com/redscaresu/github-actions` returns a score whose `repo.commit` equals that run's SHA within an hour; VERIFIED if it does not and the job log carries scorecard-action's publish refusal/warning. A follow-up PR removes both probe files from `main` in every outcome. Every `uses:` in the probe is a 40-hex SHA with `# vX.Y.Z`; `zizmor .github/` at the default persona reports 0 with no `# zizmor: ignore`; `gh api repos/redscaresu/github-actions/tags` is `[]`.

**Answer:** REFUTED, https://github.com/redscaresu/github-actions/actions/runs/38005404874 (merge SHA c64c92db35954eb9761ee51a8646a2a30300943f, score 4.1 on both API hosts); the owner chose decision 6.

**Out of scope:** The five gates, fixture, dogfood ci.yml, templates (bundle-gates, bundle-templates-readme). Any consumer. Scorecard's SARIF upload (the probe needs no `security-events: write`).

**Constraints:** Scorecard publishes only from the default branch of a public repo (goldfinger scorecard.yml:7-8,44-46), so the probe merges to `main` before any ruleset exists; HLD:353's branch protection names dogfood checks that do not exist until bundle-gates. `publish_results: true`, `results_format: sarif`. Repo stays public (HLD:121-123). Pins resolved to current releases (HLD:376-378). Local check: `zizmor --offline .github/`.

**Depends on:** none. **Areas:** [github-actions] .github/workflows/probe-scorecard.yml, [github-actions] .github/workflows/scorecard.yml (probe copy, removed after), [github-actions] README.md (## Scorecard).

**Your part:** Merge the probe PR to `main` (no protection yet) and the removal PR. Read the result with the lead (API or log). If REFUTED, confirm decision 6 (Scorecard becomes a bundle job) or choose the template anyway. Then set bundle-gates, bundle-templates-readme and public-only-gates `active` with that answer applied. When approving these epics, confirm the `decisions` list in this file; decision 6 is re-confirmed after the probe. The lead learns the answer from the API/log and the go-ahead from the board.
