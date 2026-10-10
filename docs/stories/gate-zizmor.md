---
kind: code
repo: github-actions
status: ready
epic: bundle-gates
depends_on: []
touches: [.github/workflows/security.yml, .github/workflows/ci.yml, go.mod, fixture/main.go]
risk: high
---

# Bundle skeleton: Go fixture, security.yml with the zizmor gate, dogfood ci.yml

Root go.mod (`module github.com/redscaresu/github-actions`, `go 1.26`, no deps) and fixture/main.go (`package main`, a dozen lines), per decision 1 (HLD:412). New .github/workflows/security.yml: `workflow_call` only; input `zizmor` (boolean, default true); workflow `permissions: {}`; no env/defaults. Job `zizmor`: `if: inputs.zizmor && (github.event_name == 'push' || github.event_name == 'pull_request')` (HLD:171); `permissions: {contents: read}` (private consumers need it, survey fact 3); checkout with persist-credentials: false; zizmorcore/zizmor-action pinned by SHA with `version: 1.30.1` and `advanced-security: false`, so the job fails on findings and uploads no SARIF (survey facts 2-3, epic:12); a comment says Dependabot does not bump `version:`. New .github/workflows/ci.yml: `pull_request` and `push: branches: [main]`; workflow `permissions: {}`; job `security` with `name: security`, `uses: $/.github/workflows/security.yml` (zizmor flags `./`), `permissions: {contents: read}` (gate-codeql widens it). Pins per survey fact 1. The local check is the epic's `check:` (amend-epic-bundle-gates); no actionlint, and a concrete need for it stops the story. Red proof: push one commit that tag-pins zizmorcore/zizmor-action (`@v0.6.4`; zizmor wants hash pins outside actions/ and github/), read the red run, then drop the commit with `git push --force-with-lease` to its parent, never a revert. CI's zizmor-action (online audits on) is the authority over local `--offline`.

**Done when:** The epic's Checks before every stage pass at the PR head. The latest pull_request run has `security / zizmor` success with its zizmor-action step success (not skipped). The red-proof run (URL in the PR body) has `security / zizmor` failure where the failing step is the zizmor-action step (checkout success) and its log names `unpinned-uses`; after merge `git log origin/main --format=%H | grep -c <red sha>` is 0. security.yml: only trigger workflow_call; inputs exactly `zizmor` (boolean, default true); job zizmor permissions exactly {contents: read}; zizmor-action has `version: 1.30.1` and `advanced-security: false`. ci.yml: triggers exactly pull_request and push to main; job `security` (`name: security`) grants exactly {contents: read}. The diff adds only the four touched files.
