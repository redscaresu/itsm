---
status: active
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
check: make check
---

# itsm's existing ci.yml is hardened in place: the five action refs SHA-pinned with `# vX.Y.Z` comments and `persist-credentials: false` on both checkouts, so zizmor at the default persona reports zero findings. No behaviour change (HLD Rollout 2). Also the precondition for adopt-bundle-private: the bundle's zizmor audits all of itsm's workflows.

**Done when:** `zizmor .github/workflows/ci.yml` at the default persona reports 0 findings (today 5 unpinned-uses + 2 artipacked, HLD:19-21) with no `# zizmor: ignore` comment; each of the five `uses:` at ci.yml:15,16,27,35,36 is a 40-hex SHA followed by `# vX.Y.Z`; both checkouts carry `with: persist-credentials: false`; job ids are still `check` and `e2e` (ci.yml:12,32) and their steps are otherwise unchanged; `check` and `e2e` green on the PR and on the `main` run after merge.

**Out of scope:** Any new workflow or job. `-mod=readonly` and a `go mod tidy` gate (HLD:252-255). Pedantic-persona findings (HLD:252-253). The unverified Playwright/Chromium download at ci.yml:42 (accepted risk, HLD:335-339).

**Constraints:** Job ids `check` and `e2e` become required-check names and must not be renamed (HLD:265). ci.yml keeps mirroring `make check` / `make e2e` (Makefile:24,27). Workflow-level `permissions: contents: read` stays (ci.yml:8-9). Pins resolved to current releases (HLD:376-378). Local check: `make check` plus `zizmor .github/workflows/ci.yml`.

**Depends on:** none. **Areas:** .github/workflows/ci.yml.

**Your part:** Read the `.github/` diff and merge (HLD:278-284). The lead sees the merge.
