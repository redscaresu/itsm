<!-- template: SECURITY.md @unreleased -->
# Security Policy

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.
Instead, report privately via one of:

- GitHub's [private vulnerability reporting](https://github.com/redscaresu/itsm/security/advisories/new) (preferred — keeps the entire thread off public timelines).
- Email: `ukashouri@gmail.com` with subject prefix `[security] itsm:`.

Please include:

- A description of the issue and its impact.
- Steps to reproduce (or a proof-of-concept).
- Affected commit / version / branch.

## Handling of credentials

itsm holds one credential, the TypeSafe API key. It is read only from the
`TYPESAFE_API_KEY` environment variable (`cmd/itsm/main.go:95`), sent only as
the `Authorization: Bearer` header to jev (`internal/jev/client.go:130`), and
never logged (`internal/server/server.go:17`). Keep it as the README's
"Key handling" section says: in the macOS Keychain, loaded into one shell at a
time. CI needs no key; the e2e walkthrough uses the fake value `e2e-fake-key`
(`internal/demo/walkthrough_e2e_test.go:97`).

A key seen on any public branch is rotated, never only removed.

## Maintainer review

GitHub's "require approvals" cannot be used with one maintainer (you cannot
approve your own PR), so the review rule is written down instead: **the owner
reads every diff touching `.github/`, `.gitleaks.toml`, `.golangci.yml` or
adding `#nosec` / `//nolint` / `//gosec:disable` / `zizmor: ignore` before
merging**, and a fork PR that touches them is not merged without that read.
