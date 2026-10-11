---
kind: lead
status: ready
epic: tag-v1-0-0
depends_on: []
touches: []
risk: high
---

# Lead reads the `# v1.0.0` diff (owner-delegated), re-runs online zizmor 1.30.1 at PR head, merges, checks main and re-runs `make hooks`

The epic's read and merge, delegated to the lead 2026-10-11: no **You:** wait. Status ready, no depends_on: it gates caller-comment-v1-0-0's merge, so blocking on it would deadlock (lead-read-adopt precedent); it starts when that PR is in review. (1) Read the whole diff: the five comment lines and the story-file deletion only; the `uses:` SHA is X; .gitleaks.toml changes line 1 only. (2) zizmor online at PR head: `zizmor --version` is `zizmor 1.30.1` (bundle security.yml:55); `GH_TOKEN=$(gh auth token) zizmor .github/` (token only in the lead's own shell, never a file, PR body, comment or agent) prints `No findings to report`. The red proof is push-tag-v1-0-0's scratch run; cite it, do not repeat it. If the real line is flagged: stop, no merge, no `zizmor: ignore`, the tag stays; a new lead story decides. (3) PR checks as caller-comment-v1-0-0 done_when (5). (4) Comment 'Lead read (owner-delegated 2026-10-11, HLD:278-284): <checks>; zizmor 1.30.1 online at head: No findings to report; scratch check after the tag (push-tag-v1-0-0): clear at X, ref-version-mismatch at 906216ee0b304aa7bf93e598ec159e36047259a1', then `gh pr merge` (never --admin), `swarm.sh reconcile`. Changes go to the builder via `swarm.sh tell`; read the new head. (5) After merge in ~/Work/itsm (lead decision 2026-10-11): `git pull --ff-only`; `make hooks` (the installed hook is a copy of scripts/pre-commit, Makefile:28, and its header just changed); check it. (6) Check the push runs at the merge SHA. Any red becomes a fix story.

**Done when:** `gh pr view <caller-comment PR> --json comments,mergedAt` shows the lead-read comment created before mergedAt containing `zizmor 1.30.1`, `No findings to report` and `ref-version-mismatch`; no comment matches `gh[pousr]_`. The lead's log shows `zizmor --version` and the online output at head. Before merge, at PR head: `security / zizmor`, `security / gitleaks`, `security / govulncheck`, `security / dependency-review` success, `security / codeql` skipped, `check` and `e2e` success. Push runs at the merge SHA on main: Security's zizmor, gitleaks, govulncheck success, codeql and dependency-review skipped (codeql: false, bundle security.yml:114; dependency-review is PR-only, :148); CI `check` and `e2e` success. In ~/Work/itsm after `make hooks`: `cmp scripts/pre-commit .git/hooks/pre-commit` exits 0 and `sed -n 2p .git/hooks/pre-commit` is `# template: pre-commit @v1.0.0`.
