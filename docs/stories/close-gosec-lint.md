---
kind: lead
status: blocked
blocked_by: [owner-read-gosec]
epic: gosec-lint
depends_on: [owner-read-gosec]
touches: [docs/epics/gosec-lint.md, docs/hld/2026-10-09-harden-the-ci-pipeline-for-going-public.md, docs/epics/go-public.md, docs/epics/adopt-bundle-private.md, docs/epics/bundle-templates-readme.md, docs/epics/public-only-gates.md, docs/epics/tag-v1-0-0.md, docs/epics/pin-ci-actions.md, docs/stories/close-gosec-lint.md]
---

# Close gosec-lint: carry `#nosec` and the review-rule fix into the HLD and later epics

One itsm docs PR, the closing PR, after the merge (owner approved the epic-text changes 2026-10-10). Rebase on main first; if pin-ci-actions has closed, leave its file alone. Delete docs/epics/gosec-lint.md and this story; mark gosec-lint done in the HLD `## Epics`, linking the gosec-lint-config PR. HLD: the gosec paragraph (:109, :113-118) records the outcome (gosec on in the lint job, two `#nosec` sites, reasons required) and drops 'from a flag' at :115; it states that golangci-lint v2.14.0's gosec ignores bare `//nosec` and only `// #nosec CODE -- reason` suppresses. The review rule (:276-282 today) and decision 7 name `#nosec`, `//nolint` and `//gosec:disable`. In go-public.md, adopt-bundle-private.md (:9 SECURITY.md literal, :17), bundle-templates-readme.md (:8, :16), public-only-gates.md, tag-v1-0-0.md and pin-ci-actions.md (if still present): `//nosec` → `#nosec`, and the review-rule citation → the edited HLD's actual line range.

**Done when:** after the closing PR merges, docs/epics/gosec-lint.md is gone and the HLD gosec-lint entry reads done and links the PR. Every gosec suppression example in `docs/epics docs/hld` is `// #nosec CODE -- reason`, except the one HLD sentence saying bare `//nosec` does not suppress. `grep -rn 'HLD:273-277' docs/epics docs/hld` is empty, and the cited range in each epic starts at the HLD's 'What a PR *can* still do' paragraph and ends at 'CODEOWNERS is not used' on the PR head. The review rule names `#nosec`, `//nolint` and `//gosec:disable`. The PR changes only files under docs/.
