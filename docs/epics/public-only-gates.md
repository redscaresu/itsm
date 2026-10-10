---
status: later
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
check: make check
---

# itsm runs the two gates that need a public repo: `codeql` switched on in the caller (its own PR, ship-switched-off rule), and Scorecard through a new `.github/workflows/scorecard.yml` caller of the bundle's reusable scorecard.yml (only that caller grants `id-token: write`; its own PR; check name `security / scorecard`) with the README badge, publishing a score; neither changes the ruleset (HLD Goal 1; Rollout 6; decisions 2, 3, 6, 9).

**Done when:** itsm `.github/workflows/security.yml` has no `codeql:` input; a `pull_request` run shows `security / codeql` conclusion `success` (not skipped); `gh api repos/redscaresu/itsm/code-scanning/analyses --jq '[.[]|select(.tool.name=="CodeQL")]|length'` > 0. itsm `.github/workflows/scorecard.yml` is a caller: `on:` push to main and a weekly schedule only, `permissions: {}`, one job `security` (`name: security`), `uses: redscaresu/github-actions/.github/workflows/scorecard.yml@<sha> # vX.Y.Z`, `permissions: {contents: read, id-token: write}`; `grep -l id-token .github/workflows/` lists only that file; the push run on `main` after its merge has job `security / scorecard` conclusion `success` (not skipped; PR runs prove nothing, a skipped job reports success) with no publish refusal/warning in its log; `curl -s https://api.securityscorecards.dev/projects/github.com/redscaresu/itsm` and `curl -s https://api.scorecard.dev/projects/github.com/redscaresu/itsm` both return a score whose `repo.commit` is that merge SHA (checked after the push run, not the weekly one); README.md's header block carries the badge `https://api.securityscorecards.dev/projects/github.com/redscaresu/itsm/badge` linking to `https://scorecard.dev/viewer/?uri=github.com/redscaresu/itsm`. `gh api repos/redscaresu/itsm/rules/branches/main` still lists exactly the seven contexts from go-public; `security / scorecard` is not among them.

**Out of scope:** Any ruleset change (go-public owns it; Scorecard is not required, decision 2). Migrating goldfinger or any other repo (HLD:371-372). Release tooling. Changing the bundle.

**Constraints:** Switch-on is its own PR after the flip, never bundled with code (method.md § Ship code switched off; HLD:368). The scorecard.yml caller has no workflow-level env/defaults (Scorecard logs a refused publish as a warning and stays green, goldfinger scorecard.yml:19-24), and Scorecard publishes only from a push or scheduled run on the default branch, so `security / scorecard` is never a PR gate. CodeQL blocks only at or above Check Failure severity (HLD:279-281). Both PRs touch `.github/` (owner-read). Local check: `make check`.

**Depends on:** go-public. **Areas:** .github/workflows/security.yml, README.md.

**Your part:** Read and merge the codeql PR, then the scorecard PR (HLD:273-277). Nothing in Settings. The lead verifies with the API calls in Done when.
