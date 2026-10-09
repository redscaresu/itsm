---
status: active
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
check: make check
---

# gosec runs in itsm's existing lint job via .golangci.yml with goldfinger's uncapped-issues settings, `_test.go` excluded for gosec, and every production finding triaged with `//nosec CODE -- reason` (HLD Design §1 gosec paragraph, §4; part of Rollout 3 but independent of the bundle).

**Done when:** `golangci-lint run` exits 0 with `gosec` in `linters.enable`, `issues.max-issues-per-linter: 0`, `issues.max-same-issues: 0`, and a `path: _test\.go` exclusion for gosec in .golangci.yml; the set of `//nosec` sites (`grep -rn '//nosec' --include='*.go' .`) equals what gosec reports on production files with `-tags e2e,live`, each with `-- reason` (expected demodata.go:80 G304 and handlers_customer.go:263 G710; handlers_customer.go:127 is checked, not assumed); no `//nosec` without a reason; `make check` green; `check` green on `main`.

**Out of scope:** Changing itsm's `default: standard` set or copying goldfinger's `default: none` config (survey-code: only the gosec pieces transfer). Any workflow change. Fixing G304 (caller-supplied demo path is by design).

**Constraints:** `build-tags: e2e, live` (.golangci.yml:3-6) lints the e2e harness, so the `_test.go` exclusion is what keeps gosec's file/subprocess noise out (HLD:256-259). G710 at :263 is a false positive: ticketPath(id) = "/tickets/" + id (handlers_customer.go:131) cannot start with `//`; the `//nosec` reason says so. Local check: `make check`.

**Depends on:** none. **Areas:** .golangci.yml, internal/demodata/, internal/server/.

**Your part:** Read the `.golangci.yml` and `//nosec` diff and merge (HLD:273-277). The lead sees the merge.
