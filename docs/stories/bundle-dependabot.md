---
kind: chore
repo: github-actions
status: blocked
blocked_by: [ruleset-and-pinning]
epic: bundle-gates
depends_on: [ruleset-and-pinning]
touches: [.github/dependabot.yml]
---

# Add the bundle's own dependabot.yml, through the ruleset

After ruleset-and-pinning, so its own PR and every bump it starts go through the ruleset and none race the gate PRs (review, critic). .github/dependabot.yml, version 2: `gomod` and `github-actions`, each directory `/`, weekly, one group with patterns ['*'], an `open-pull-requests-limit`. No auto-merge: no dependabot/fetch-metadata, no `gh pr merge --auto`. Header comment: the owner reads every bump before merging it through the ruleset. Based on goldfinger dependabot.yml:1-26 without 'org-wide write credentials' (survey fact 14). If zizmor flags the file (e.g. dependabot-cooldown), fix it, never ignore.

**Done when:** The epic's Checks pass at the PR head (zizmor audits dependabot.yml) and the PR merged with every required check green. dependabot.yml has exactly two `updates` entries, `gomod` and `github-actions`, each with directory `/`, `schedule.interval: weekly`, a `groups:` entry and `open-pull-requests-limit`; its header comment says the owner reads every bump. `grep -rn 'fetch-metadata\|merge --auto\|org-wide' .github/` is empty. After merge, `gh run list --repo redscaresu/github-actions --event dynamic --json displayTitle,conclusion` shows Dependabot Updates runs for gomod and github_actions, each `success` (a rejected config never reaches success). The diff adds only .github/dependabot.yml.
