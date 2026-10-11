---
kind: chore
status: ready
epic: go-public
depends_on: []
touches: [LICENSE, docs/stories/add-license.md]
---

# itsm: add LICENSE, a byte-identical copy of github-actions' Apache-2.0 LICENSE

One itsm PR: `cp ~/Work/github-actions/LICENSE LICENSE` at the worktree root (stock Apache-2.0, 11357 bytes, sha256 c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4; survey). Keep the appendix placeholder `[yyyy] [name of copyright owner]` (../github-actions/LICENSE:189) as is: byte-identical is the requirement (go-public.md:9). Nothing else: no README line, no file headers (go-public.md:11). The PR deletes docs/stories/add-license.md (docs/stories/README.md:7-8). Not a sensitive diff (no .github/, .gitleaks.toml, .golangci.yml or suppression; HLD:278-284), so with merge = agent (.claude/swarm/config:3) the conductor merges it when green and reviewed; no lead-read story. ci.yml:3-6 and security.yml:3-8 have no paths filter, so all seven checks report on a LICENSE-only PR.

**Done when:** (1) `git diff --name-status origin/main...HEAD` is exactly `A LICENSE` and `D docs/stories/add-license.md`. (2) `cmp LICENSE ~/Work/github-actions/LICENSE` exits 0; `shasum -a 256 LICENSE` prints c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4; `wc -c < LICENSE | tr -d ' '` prints 11357. (3) `make check` passes at head. (4) PR checks at head: `check`, `e2e`, `security / zizmor`, `security / gitleaks`, `security / govulncheck`, `security / dependency-review` success; `security / codeql` skipped. (5) After the merge: LICENSE on origin/main passes the same cmp, and the push runs at the merge SHA, `CI` and `Security`, conclude success.
