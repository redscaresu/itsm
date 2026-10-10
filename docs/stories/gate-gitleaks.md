---
kind: code
repo: github-actions
status: blocked
blocked_by: [gate-zizmor]
epic: bundle-gates
depends_on: [gate-zizmor]
touches: [.github/workflows/security.yml]
risk: high
---

# Add the gitleaks gate: checksum-verified binary, full history, redacted, verbose

Adds input `gitleaks` (boolean, default true) and job `gitleaks` to security.yml. `if:` inputs.gitleaks and event push, pull_request or schedule (HLD:172). `permissions: {contents: read}`. Checkout `fetch-depth: 0`, persist-credentials: false. One run step: download gitleaks_8.30.1_linux_x64.tar.gz into $RUNNER_TEMP, `echo '551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb  <file>' | sha256sum -c -` before `tar`, then `gitleaks detect --redact -v --no-banner --source=.`. `-v` is required: without it 8.30.1 prints no finding, so REDACTED, RuleID and File never reach the log (skeptic-negative-control; survey fact 17 used -v). Binary, not gitleaks-action (EULA, PR comments need more than contents: read; survey facts 4-5). A comment says Dependabot bumps neither the version nor the sha256. Test locally whether 8.30.1 loads a repo-root .gitleaks.toml without `--config` (an allowlist for the proof file silences it); if not, the step passes `--config .gitleaks.toml` when the file exists. Red proof: one commit adds the jwt.io example token to fixture/gitleaks-proof.txt; read the red run; drop it by force-push (gitleaks scans full history and merge commits are allowed, so a revert would leave it on main). If push protection blocks the push, never bypass it; pick another documented-example value the default ruleset flags.

**Done when:** The epic's Checks pass at the PR head. Latest PR run: `security / gitleaks` success, its gitleaks step success (not skipped), log shows more than one commit scanned and `no leaks found`. Red-proof run (URL in PR body): `security / gitleaks` failure at the gitleaks step with the `sha256sum -c` line `OK`; its log has `RuleID:` with `jwt`, `File:` with `fixture/gitleaks-proof.txt`, and `REDACTED`; `gh run view <red run> --log | grep -c SflKxwRJ` is 0; the red SHA is not in `main`'s history after merge. Locally, the step's download-and-verify lines exit non-zero before `tar` with one sha256 hex digit changed. security.yml contains `gitleaks detect --redact -v`, `fetch-depth: 0` and the 64-hex sha256, and no `gitleaks/gitleaks-action`. Job gitleaks grants exactly {contents: read}; inputs are exactly zizmor and gitleaks. The PR body records the .gitleaks.toml auto-load result and, if `--config` was needed, a local run showing it applied.
