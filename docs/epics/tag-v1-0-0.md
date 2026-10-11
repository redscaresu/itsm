---
status: later
hld: 2026-10-09-harden-the-ci-pipeline-for-going-public
---

# redscaresu/github-actions gets its first version: an annotated tag `v1.0.0` on X, the exact SHA itsm's caller pins, cut only after the dogfood is green at X and itsm's `security / *` run at X is green; then one itsm PR adds `# v1.0.0` to the caller line (which carries no comment before the tag) and rewrites `unreleased` to `v1.0.0` in the four template headers, then confirms online zizmor reports no ref-version-mismatch (HLD Goal 3; Design §2; Rollout 4). Kept as its own epic: it is the hand step between the two repos.

**Done when:** Let X be the 40-hex in the `uses:` line of itsm `.github/workflows/security.yml` on `main`. Preconditions, checked before tagging: `gh run list --repo redscaresu/github-actions --workflow ci.yml --commit X --json conclusion` is non-empty and all `success`; an itsm run (PR or `main`) at pin X has `security / zizmor`, `security / gitleaks`, `security / govulncheck`, `security / dependency-review` `success`; bundle-scorecard-probe is done. Tag: `git ls-remote https://github.com/redscaresu/github-actions 'refs/tags/v1.0.0^{}'` prints X; `gh api repos/redscaresu/github-actions/git/ref/tags/v1.0.0 --jq .object.type` is `tag`. itsm PR: the `uses:` line ends `@X # v1.0.0`; the four template headers read `@v1.0.0`; nothing else changes; its `security / zizmor` is green; the `main` run after merge is green.

**Out of scope:** Any github-actions file change (the versioning README is already at X). A release workflow (HLD:232-233). A moving `v1` (HLD:230-231). Re-pinning itsm to a newer SHA.

**Constraints:** Annotated, by hand, from a green dogfood (HLD:59,232-233,360); the tag object's SHA differs from X, so compare the peeled ref, never `git/ref/tags` `.object.sha`. If adoption forced a bundle fix, X is the fixed SHA, not Rollout 1's (HLD:360 says 'the step-1 SHA'). Dependabot opens no PR for a comment-only change, so the rewrite is a hand PR touching `.github/` (owner-read, HLD:278-284,361). MAJOR/MINOR/PATCH rules HLD:223-225.

**Depends on:** adopt-bundle-private. **Areas:** [github-actions] tags, .github/workflows/security.yml, .github/dependabot.yml, .gitleaks.toml, SECURITY.md, scripts/pre-commit.

**Your part:** When the lead reports both preconditions green: in ~/Work/github-actions, `git fetch origin && git tag -a v1.0.0 X -m 'v1.0.0'` and `git push origin v1.0.0`, X read from itsm's caller line. Tell the lead; it checks the peeled ref. Then read and merge the lead's itsm `# v1.0.0` PR.
