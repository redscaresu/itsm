---
kind: lead
status: ready
epic: pin-ci-actions
depends_on: []
touches: []
risk: high
---

# Owner reads the ci.yml diff; lead runs the online audit, merges and verifies main

Starts once pin-ci-actions-shas is in review (not blocked on its merge: this story gates it). The lead first runs `GH_TOKEN=$(gh auth token) zizmor .github/workflows/ci.yml` at the PR head and posts the output as a PR comment. **You:** read the PR — the ci.yml diff (5 pins, 2 `persist-credentials: false`), the offline zizmor before/after in its body and the lead's online comment — and tell the lead go, or what to change. The lead posts your go as a PR comment, merges (merge = agent), runs `swarm.sh reconcile`, then checks main. Any red becomes a fix story. Epic text approved by the owner 2026-10-11.

**Done when:** the lead's online-zizmor comment (`No findings to report`) and the comment quoting your go both predate mergedAt; the ci.yml push run for the merge SHA has `check` and `e2e` success; on main, `zizmor --offline .github/workflows/ci.yml` and `GH_TOKEN=$(gh auth token) zizmor .github/workflows/ci.yml` both print `No findings to report`.
