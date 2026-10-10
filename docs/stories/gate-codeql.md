---
kind: code
repo: github-actions
status: blocked
blocked_by: [gate-govulncheck]
epic: bundle-gates
depends_on: [gate-govulncheck]
touches: [.github/workflows/security.yml, .github/workflows/ci.yml]
risk: high
---

# Add the codeql gate and the codeql-off dogfood caller

Adds inputs `codeql` (boolean, default true) and `codeql-languages` (string, default `go`). Job `codeql`: `if:` inputs.codeql and event push, pull_request or schedule; `permissions: {contents: read, security-events: write}` (job permissions replace defaults; no `actions: read`, no consumer grants it). Steps: checkout (persist-credentials: false); setup-go (go-version-file go.mod, cache false, `if: hashFiles('go.mod') != ''`); codeql-action init (`languages: ${{ inputs.codeql-languages }}`, build-mode autobuild); analyze (category `/language:${{ inputs.codeql-languages }}`), as goldfinger codeql.yml:37-53. Pin the v4.x tag, not releases/latest (survey fact 1). ci.yml: `security` grants exactly {contents: read, security-events: write}; new job `security-codeql-off` (no `name:`), same `uses:`, `with: codeql: false`, same grant (a caller covers every called job, skipped ones included; survey fact 12). Red proof: one commit adds to fixture/main.go an `http.HandleFunc` handler that passes `r.URL.Query().Get("c")` to `exec.Command`; read the run; drop it by force-push. `security / codeql` stays success there: analyze uploads and does not fail on alerts; the blocking signal is the code-scanning check `CodeQL` at Check Failure severity (HLD:284-286), which ruleset-and-pinning requires (contradiction 6). Do not click default setup.

**Done when:** The epic's Checks pass at the PR head. Latest PR run: `security / codeql` success with its analyze step success (not skipped); `security-codeql-off / codeql` skipped while `security-codeql-off / zizmor`, `/ gitleaks`, `/ govulncheck` are success. `gh api 'repos/redscaresu/github-actions/code-scanning/analyses?ref=refs/pull/<n>/merge' --jq '[.[]|select(.tool.name=="CodeQL")]|length'` > 0. Red proof, recorded in the PR body before the drop: on the red SHA the check run `CodeQL` has conclusion failure, and `gh api 'repos/redscaresu/github-actions/code-scanning/alerts?ref=refs/pull/<n>/merge' --jq '[.[].rule.id]'` contains `go/command-injection`; the red SHA is not in `main`'s history after merge. `code-scanning/default-setup --jq .state` is `not-configured`. Job codeql grants exactly {contents: read, security-events: write}, every other security.yml job exactly {contents: read}; both ci.yml callers exactly {contents: read, security-events: write}. Every codeql-action ref carries `# v4.`.
