---
kind: lead
repo: github-actions
status: ready
epic: bundle-gates
depends_on: [bundle-dependabot]
touches: []
risk: high
---

# Negative-control PR proves gitleaks, govulncheck and dependency-review fire on main's bundle; owner tries the graph toggle and closes it unmerged

Needs your go-ahead: the PR's refs/pull ref is permanent and public (decision 8). After bundle-dependabot. The lead works in ~/Work/github-actions on branch `negative-control` from current main. Commit 1 adds fixture/negative-control.txt with the jwt.io documented example token (gitleaks rule `jwt`, not a partner pattern; survey fact 17). Commit 2 adds `require golang.org/x/text v0.3.6`, its go.sum lines and a `language.ParseAcceptLanguage` call in fixture/main.go (survey fact 16). If push protection blocks the push, never bypass it; pick another documented-example value the default ruleset flags. The lead opens PR 'negative control: do not merge'. **You:** read it and confirm the three red gates. **You:** Settings → Code security, try to turn the dependency graph off (likely not offered on a public repo, review). If it goes off, the lead re-runs `security / dependency-review` and confirms the guard's public-repo `::error::`, then **You:** turn it back on (and alerts and security updates if they went off). If it cannot, say so; the lead records that the 403-public branch is covered only by gate-dependency-review's stub test. **You:** close the PR unmerged.

**Done when:** On the PR's pull_request run, `security / gitleaks`, `security / govulncheck`, `security / dependency-review` and the same three under `security-codeql-off /` have conclusion failure; `security / zizmor` is success. The `security / gitleaks` log has `RuleID:` with `jwt`, `File:` with `fixture/negative-control.txt`, and `REDACTED`, and `gh run view <run> --log | grep -c SflKxwRJ` is 0. The govulncheck log names GO-2021-0113; the dependency-review log names GHSA-ppp9-7jff-5vj2. While open, `gh pr view <n> --json mergeStateStatus` is `BLOCKED`. The graph-test outcome is recorded in your words: the `::error::` line, or 'cannot be turned off'. Finally `gh pr view <n> --json state,mergedAt` is CLOSED with mergedAt null, and code-security-settings' checks still hold with `dependabot_security_updates` `enabled`.
