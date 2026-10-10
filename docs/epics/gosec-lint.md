---
status: active
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
check: make check
---

# gosec runs in itsm's existing lint job via .golangci.yml with goldfinger's uncapped-issues settings, `_test.go` excluded for gosec, and every production finding triaged with a trailing `// #nosec CODE -- reason` on the flagged line, and gosec's `nosec-require-justification` on so a reasonless `#nosec` fails lint (owner, 2026-10-10) (HLD Design §1 gosec paragraph, §4; part of Rollout 3 but independent of the bundle).

**Done when:** `golangci-lint run` exits 0 with `gosec` in `linters.enable`, `issues.max-issues-per-linter: 0`, `issues.max-same-issues: 0`, and a `path: _test\.go` exclusion for gosec in .golangci.yml; the gosec suppression sites (`grep -rnE '//nosec|#nosec|gosec:disable|nolint' --include='*.go' .`) are exactly `// #nosec G304 -- <reason>` at demodata.go:80 and `// #nosec G710 -- <reason>` at handlers_customer.go:263, plus the existing `//nolint:noctx` at internal/demo/server.go:33; with either #nosec removed, gosec reports exactly that finding and none in `_test.go` (handlers_customer.go:127 was checked and does not flag); a reasonless `// #nosec G304` makes `golangci-lint run` report it; `make check` green; `check` green on `main`.

**Out of scope:** Changing itsm's `default: standard` set or copying goldfinger's `default: none` config (survey-code: only the gosec pieces transfer). Any workflow change. Fixing G304 (every caller passes the constant demodata.File or a test fixture; no request input reaches it).

**Constraints:** `build-tags: e2e, live` (.golangci.yml:3-6) lints the e2e harness, so the `_test.go` exclusion is what keeps gosec's file/subprocess noise out (HLD:261-264). G710 at :263 is a false positive: ticketPath(id) = "/tickets/" + id (handlers_customer.go:131) cannot start with `//`; the `#nosec` reason says so. golangci-lint v2.14.0's gosec ignores bare `//nosec`; only `#nosec` suppresses (survey, 2026-10-10). Other suppressions (`//nolint`, `//gosec:disable`) also silence gosec, so the review rule names them. Local check: `make check`.

**Depends on:** none. **Areas:** .golangci.yml, internal/demodata/, internal/server/.

**Your part:** Read the `.golangci.yml` and `#nosec` diff and tell the lead go; the lead merges (merge = agent) (HLD:276-282). Approved 2026-10-10: the closing PR changes `//nosec` to `#nosec` in approved epics and names `//nolint` and `//gosec:disable` in the review rule.
