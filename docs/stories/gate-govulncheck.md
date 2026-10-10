---
kind: code
repo: github-actions
status: blocked
blocked_by: [gate-gitleaks]
epic: bundle-gates
depends_on: [gate-gitleaks]
touches: [.github/workflows/security.yml]
---

# Add the govulncheck gate at a pinned version

Adds input `govulncheck` (boolean, default true) and job `govulncheck` to security.yml. `if:` inputs.govulncheck and event push, pull_request or schedule. `permissions: {contents: read}`. Steps: checkout (persist-credentials: false); setup-go `go-version-file: go.mod`, `cache: false` (no go.sum); `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` (module proxy + sumdb verify it; govulncheck-action installs @latest, survey fact 6). The Go-only guard `if: hashFiles('go.mod') != ''` goes on the setup-go and govulncheck steps (contradiction 9). Red proof: one commit adds `require golang.org/x/text v0.3.6`, its go.sum lines and a `language.ParseAcceptLanguage("en")` call in fixture/main.go (survey fact 16); read the red run; drop it by force-push.

**Done when:** The epic's Checks pass at the PR head. Latest PR run: `security / govulncheck` success, govulncheck step success (not skipped), log says 'No vulnerabilities found'. Red-proof run (URL in PR body): `security / govulncheck` failure at the govulncheck step (setup-go success) and its log names GO-2021-0113; the red SHA is not in `main`'s history after merge. `grep -rn '@latest\|govulncheck-action' .github/` is empty. Locally `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` exits 0. Inputs exactly zizmor, gitleaks, govulncheck; the job grants exactly {contents: read}.
