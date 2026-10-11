---
status: active
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
check: make check
---

# itsm's existing ci.yml is hardened in place: the five action refs SHA-pinned with `# vX.Y.Z` comments and `persist-credentials: false` on both checkouts, so zizmor at the default persona reports zero findings. No behaviour change (HLD Rollout 2). Also the precondition for adopt-bundle-private: the bundle's zizmor audits all of itsm's workflows.

**Done when:** zizmor 1.30.1 at the default persona reports 0 findings on .github/workflows/ci.yml both offline (`--offline`, before/after in the PR body) and online (the lead runs it with GH_TOKEN at the PR head and on `main`, matching the bundle's zizmor-action `online-audits: true`) (today 5 unpinned-uses + 2 artipacked, HLD:19-21) with no `# zizmor: ignore` comment; each of the five `uses:` at ci.yml:15,16,27,35,36 is a 40-hex SHA followed by `# vX.Y.Z`; both checkouts carry `with: persist-credentials: false`; job ids are still `check` and `e2e` (ci.yml:12,32) and their steps are otherwise unchanged; `check` and `e2e` green on the PR and on the `main` run after merge.

**Out of scope:** Any new workflow or job. `-mod=readonly` and a `go mod tidy` gate (HLD:260-262). Pedantic-persona findings (HLD:259-260). The unverified Playwright/Chromium download at ci.yml:42 (accepted risk, HLD:343-347).

**Constraints:** Job ids `check` and `e2e` become required-check names and must not be renamed (HLD:272-273). ci.yml keeps mirroring `make check` / `make e2e` (Makefile:24,27). Workflow-level `permissions: contents: read` stays (ci.yml:8-9). Pins resolved to current releases (HLD:385-387; the commit the current floating major resolves to, never a major bump). Local check: `make check` plus `zizmor .github/workflows/ci.yml`.

**Depends on:** none. **Areas:** .github/workflows/ci.yml.

**Your part:** Read the `.github/` diff and tell the lead go; the lead merges (merge = agent) (HLD:278-284). Epic text approved 2026-10-11.
