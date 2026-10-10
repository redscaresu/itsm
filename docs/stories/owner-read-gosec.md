---
kind: lead
status: ready
epic: gosec-lint
depends_on: []
touches: []
risk: high
---

# Owner reads the gosec PR; lead merges and verifies main

Starts once gosec-lint-config is in review (not blocked on its merge: this story gates that merge). **You:** read the gosec-lint-config PR — the .golangci.yml diff, the two `// #nosec` lines with their reasons, and the scratch proofs — and tell the lead go, or what to change (the lead passes changes to the builder; you read the new head). The lead posts your go as a PR comment, merges (merge = agent), runs `swarm.sh reconcile`, then checks main. Any red on main becomes a fix story.

**Done when:** the PR's comments show the lead's comment quoting your go, created before mergedAt; the ci.yml push run for the merge SHA has `check` and `e2e` success; on main with golangci-lint v2.14.0, `golangci-lint run` prints `0 issues.` and `make check` exits 0.
