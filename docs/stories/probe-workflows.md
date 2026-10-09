---
kind: code
repo: github-actions
status: ready
epic: bundle-scorecard-probe
depends_on: []
touches: [.github/workflows/probe-scorecard.yml, .github/workflows/scorecard.yml]
risk: high
---

# Add the Scorecard probe: reusable scorecard.yml and a push:main caller

New .github/workflows/scorecard.yml: `on: workflow_call` only; workflow `permissions: {}`; job `analysis` named `Scorecard analysis`, ubuntu-latest, `permissions: {contents: read, id-token: write}`; steps checkout (persist-credentials: false), ossf/scorecard-action (results_file results.sarif, results_format sarif, publish_results true), upload-artifact of results.sarif. That is goldfinger scorecard.yml:26-56 without upload-sarif and security-events (epic:10). New .github/workflows/probe-scorecard.yml: `on: push: branches: [main]` only; workflow `permissions: {}`; one job `scorecard` with `permissions: {contents: read, id-token: write}` and `uses: $/.github/workflows/scorecard.yml`. No env/defaults at any level. Pins re-resolved to latest releases at build (survey.json:33: scorecard-action v2.4.4 2d114668…, checkout v7.0.1 3d3c42e5…, upload-artifact v7.0.2 cf430e03…). Open the PR and stop: the owner merges it in probe-run-and-read, never the conductor.

**Done when:** On the PR branch in github-actions: `zizmor --offline .github/` prints 'No findings to report' and exits 0; `grep -rn 'zizmor: ignore' .github/` and `grep -nE '^(env|defaults):' .github/workflows/*.yml` are empty; every action `uses:` matches `@[0-9a-f]{40} # v[0-9]+\.[0-9]+\.[0-9]+` and its SHA equals `git ls-remote https://github.com/<owner>/<repo> 'refs/tags/<tag>^{}'` (the bare tag ref if lightweight) for that repo's latest release; scorecard.yml's only trigger is workflow_call and it contains no `run:`, `security-events`, `codeql-action` or `read-all`; probe-scorecard.yml's only trigger is push to main and its job grants exactly `contents: read` and `id-token: write`; the PR is open, unmerged and changes only these two files. Also: `grep -nE '^\s*(env|defaults|container|services):' .github/workflows/*.yml` is empty; scorecard.yml has exactly one job, `runs-on: ubuntu-latest`, whose action `uses:` are exactly actions/checkout, ossf/scorecard-action and actions/upload-artifact in that order, and only that job has `id-token: write`; probe-scorecard.yml's only job is `uses: $/.github/workflows/scorecard.yml` (it calls the reusable workflow, never inlines the steps).
