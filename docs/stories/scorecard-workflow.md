---
kind: code
repo: github-actions
status: blocked
blocked_by: [gate-zizmor]
epic: bundle-gates
depends_on: [gate-zizmor]
touches: [.github/workflows/scorecard.yml, .github/workflows/scorecard-dogfood.yml]
risk: high
---

# Scorecard as the bundle's reusable scorecard.yml, dogfooded by a push-only caller file; prove PR runs never request id-token

New .github/workflows/scorecard.yml: the probe file that published (~/Work/github-actions f36359e:.github/workflows/scorecard.yml:1-39; run 38005404874, HLD:148-151) with job id `scorecard` and no job `name:` (check `security / scorecard`), comments reworded: `workflow_call` only, no inputs, workflow `permissions: {}`, no env/defaults, job permissions {contents: read, id-token: write}, steps checkout (persist-credentials: false), ossf/scorecard-action (results_file results.sarif, results_format sarif, publish_results true), upload-artifact (retention-days 5). No job `if:` (contradiction 2). New .github/workflows/scorecard-dogfood.yml: the probe caller f36359e:.github/workflows/probe-scorecard.yml:5-16 with job id `security` and `name: security`: `on: push: branches: [main]` only, workflow `permissions: {}`, `uses: $/.github/workflows/scorecard.yml`, job permissions {contents: read, id-token: write}. ci.yml and security.yml are not touched, so no `pull_request` run ever contains an id-token request (skeptic-scorecard-workflow). The proof reads the `GITHUB_TOKEN Permissions` group every job's 'Set up job' log prints; the push run is the positive control for that grep.

**Done when:** The epic's Checks pass at the PR head. Real-run proof that a pull_request run never requests id-token: `gh api 'repos/redscaresu/github-actions/actions/runs?head_sha=<PR head sha>' --jq '[.workflow_runs[]|{path,event,conclusion}]'` lists only `.github/workflows/ci.yml`, event pull_request, conclusion success (no startup_failure, no scorecard-dogfood run), and `gh run view <that run> --log | grep -c IdToken` is 0. After merge, before the story closes: the scorecard-dogfood push run for the merge SHA has `security / scorecard` success with the scorecard-action step success (not skipped), its log contains `IdToken: write` and no publish refusal or warning; both `curl -s https://api.securityscorecards.dev/projects/github.com/redscaresu/github-actions` and `curl -s https://api.scorecard.dev/projects/github.com/redscaresu/github-actions` return `repo.commit` equal to the merge SHA (or a later main SHA whose dogfood push run succeeded). `grep -l 'id-token' .github/workflows/*` is exactly scorecard.yml and scorecard-dogfood.yml. scorecard.yml: only trigger workflow_call; job `scorecard` grants exactly {contents: read, id-token: write}; no `run:`, `security-events`, `codeql-action`, `env:`, `defaults:`. scorecard-dogfood.yml: `on:` exactly push to main; job `security` grants exactly {contents: read, id-token: write}. The diff adds only these two files.
