---
kind: docs
repo: github-actions
status: blocked
blocked_by: [probe-run-and-read]
epic: bundle-scorecard-probe
depends_on: [probe-run-and-read]
touches: [.github/workflows/probe-scorecard.yml, .github/workflows/scorecard.yml, README.md]
risk: high
---

# Record the Scorecard answer in README and remove the probe

Starts once probe-run-and-read has written the verdict and run URL into this file. Delete .github/workflows/probe-scorecard.yml and .github/workflows/scorecard.yml in every outcome (one PR, so no extra publish run fires between answer and removal). Add README.md `## Scorecard` after the existing two lines: the verdict word; one sentence on its meaning (VERIFIED: Scorecard cannot publish from a workflow_call job; REFUTED: it can, shown for an intra-repo call); the run URL; the evidence; that the cross-repo case is proven later by public-only-gates' API check on itsm. Conductor merges.

**Done when:** On the PR branch `git ls-files .github/` is empty; README.md lines 1-2 are unchanged and it has exactly one `## Scorecard` heading whose section contains the verdict word and the `https://github.com/redscaresu/github-actions/actions/runs/<id>` URL exactly as recorded here by probe-run-and-read, plus the word `intra-repo`; the diff touches only README.md and the two workflow files; no tag pushed. Also: the `## Scorecard` section contains exactly one of VERIFIED|REFUTED, equal to the verdict recorded in this file; it contains the recorded evidence verbatim (the 40-hex repo.commit, or the refusal line) and the words `intra-repo` and `public-only-gates`. After merge, github-actions main has README `## Scorecard` and `git ls-files .github/` empty.
