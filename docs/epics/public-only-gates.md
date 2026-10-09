---
status: later
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
check: make check
---

# itsm runs the two gates that need a public repo: `codeql` switched on in the caller (its own PR, ship-switched-off rule), and Scorecard from `templates/scorecard.yml` with the README badge, publishing a score; neither changes the ruleset (HLD Goal 1; Rollout 6; decisions 2, 3, 9). If decision 6 moved Scorecard into the bundle, the second story instead sets `scorecard: true` and `id-token: write` in the caller (check name `security / scorecard`).

**Done when:** itsm `.github/workflows/security.yml` has no `codeql:` input; a `pull_request` run shows `security / codeql` conclusion `success` (not skipped); `gh api repos/redscaresu/itsm/code-scanning/analyses --jq '[.[]|select(.tool.name=="CodeQL")]|length'` > 0. `.github/workflows/scorecard.yml` equals `templates/scorecard.yml` plus header `# template: scorecard.yml @v1.0.0`; the push run on `main` after its merge has job `Scorecard analysis` green with no publish refusal/warning in its log; `curl -s https://api.securityscorecards.dev/projects/github.com/redscaresu/itsm` returns a score whose `repo.commit` is that merge SHA (checked after the push run, not the weekly one); analyses whose `tool.name` contains `Scorecard` > 0. README.md's header block carries the badge `https://api.securityscorecards.dev/projects/github.com/redscaresu/itsm/badge` linking to `https://scorecard.dev/viewer/?uri=github.com/redscaresu/itsm`. `gh api repos/redscaresu/itsm/rules/branches/main` still lists exactly the seven contexts from go-public; `Scorecard analysis` is not among them.

**Out of scope:** Any ruleset change (go-public owns it; Scorecard is not required, decision 2). Migrating goldfinger or any other repo (HLD:371-372). Release tooling. Changing the bundle.

**Constraints:** Switch-on is its own PR after the flip, never bundled with code (method.md § Ship code switched off; HLD:368). scorecard.yml: `permissions: read-all`, job `analysis` / `Scorecard analysis` with `id-token: write` + `security-events: write`, no workflow-level env/defaults (Scorecard logs a refused publish as a warning and stays green, goldfinger scorecard.yml:19-24), triggers push:main + schedule + branch_protection_rule only (scorecard.yml:9-14), so it can never be a PR gate. CodeQL blocks only at or above Check Failure severity (HLD:279-281). Both PRs touch `.github/` (owner-read). Local check: `make check`.

**Depends on:** go-public. **Areas:** .github/workflows/security.yml, .github/workflows/scorecard.yml, README.md.

**Your part:** Read and merge the codeql PR, then the scorecard PR (HLD:273-277). Nothing in Settings. The lead verifies with the API calls in Done when.
