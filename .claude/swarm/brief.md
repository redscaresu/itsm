Project rules (itsm):
- Codex is often out of quota. If codex is skipped for any reason, review your diff with Claude
  instead: run the /code-review skill at high effort on your branch, fix real findings, and write
  "Claude review in place of codex" with its findings in the PR body.
- Never change GitHub repository settings, rulesets, visibility or tags; those are the owner's.
- Pin every GitHub Action by full 40-hex commit SHA with a `# vX.Y.Z` comment.
