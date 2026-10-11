---
kind: lead
status: blocked
blocked_by: [owner-read-pins]
epic: pin-ci-actions
depends_on: [owner-read-pins]
touches: [docs/epics/pin-ci-actions.md, docs/hld/2026-10-09-harden-the-ci-pipeline-for-going-public.md, docs/epics/adopt-bundle-private.md, docs/stories/close-pin-ci-actions.md]
---

# Close pin-ci-actions: mark it done in the HLD and fix the ci.yml cites the change moved

One itsm docs PR after the merge. Rebase on main first and locate every edit by its text, not line number (another epic's close may have moved the HLD). Delete docs/epics/pin-ci-actions.md and this story; the HLD `## Epics` entry `[[pin-ci-actions]] (active)` becomes `pin-ci-actions (done <date>: <PR URL>)`. The two `(`ci.yml:42`)` cites become the Playwright line's new number on main (`grep -n 'playwright install' .github/workflows/ci.yml`; expected 46). The comparison-table row "Actions pinned by SHA …" gets `*done: pin-ci-actions, <PR>*` inside its right-hand cell (before the closing `|`); the `zizmor.yml` row's "none (local zizmor: 7 findings)" becomes "none (local zizmor: 0 findings after pin-ci-actions, <PR>)" — no zizmor gate exists yet. adopt-bundle-private.md: the '7 findings would redden it' clause notes it is satisfied by pin-ci-actions (<PR>). All edits in place.

**Done when:** after the closing PR merges, docs/epics/pin-ci-actions.md and the epic's story files are gone; the HLD entry reads done and links the PR; `grep -rn '\[\[pin-ci-actions\]\]\|ci.yml:42' docs/` is empty and the new cite's line on main contains `playwright install`; both table notes sit inside their cells; the HLD line count equals origin/main's at rebase; the PR changes only files under docs/.
