---
title: Harden the CI pipeline for going public
date: 2026-10-09
status: agreed
---

# Harden the CI pipeline for going public

## Problem

`redscaresu/itsm` is private today (`gh api repos/redscaresu/itsm` → `"visibility":"private"`,
`"security_and_analysis":null`) and is about to be made public. Its CI is a single workflow that
runs vet, test, lint and the headless e2e walkthrough (`.github/workflows/ci.yml`). It has none of
the supply-chain or security gates that the sibling repo `redscaresu/goldfinger` runs, and going
public changes the threat model in three ways:

1. **Anyone can open a PR.** `ci.yml` runs on every `pull_request` with actions pinned to
   floating tags (`actions/checkout@v7`, `actions/setup-go@v7`, `golangci/golangci-lint-action@v9`,
   `ci.yml:15,16,27,35,36`). A compromised tag resolves to attacker code on the next run. zizmor
   1.30.1 at the default persona on the current workflows: 5 × `unpinned-uses` (high), 2 ×
   `artipacked` (credential persistence via `actions/checkout` default `persist-credentials: true`).
2. **The code handles a real credential.** The TypeSafe API key is read from the environment
   (`cmd/itsm/main.go:95`), sent as a bearer token (`internal/jev/client.go:130`) and documented
   as never logged (`internal/server/server.go:17`, README "Key handling"). Once public, a key
   pushed to *any* branch is world-readable at push time, and GitHub keeps force-pushed commits
   reachable by SHA. There is no control before the push and no detection after it.
3. **Dependencies are unwatched.** No Dependabot config, no govulncheck, Dependabot alerts are
   off (`gh api repos/redscaresu/itsm/vulnerability-alerts` → 404). `playwright-go` pulls a
   driver and Chromium at e2e time (`ci.yml:46`); nothing checks any of it for known
   vulnerabilities.

There is also no `SECURITY.md`, so a researcher who finds something has no private channel, and
no `LICENSE`, so the public repo would be all-rights-reserved by default.

**The workflow gates should not be copied in.** goldfinger carries ~400 lines of hand-tuned
security workflows (`ci.yml` gitleaks/govulncheck jobs, `codeql.yml`, `dependency-review.yml`,
`scorecard.yml`, `zizmor.yml`). Action SHA pins inside a copy do *not* drift: each repo's own
Dependabot bumps them. What drifts is structure and workarounds: the dependency-graph guard, the
Node 24 opt-in, the Scorecard env restrictions, a new gate. Each of those is a hand edit in every
repo that copied the file. The owner has ~30 public repos (`gh repo list redscaresu`) and wants
the same posture on all of them. The decision taken in this pane: build the workflow gates once
as a **reusable workflow** (`on: workflow_call`) in `redscaresu/github-actions` (exists, public,
Apache-2.0, empty apart from LICENSE and README, checkout at `~/Work/github-actions`), and make
itsm the first consumer. The user accepts that this puts the shared repo on the critical path to
going public (Rollout).

## Goals

1. itsm goes public with every gate goldfinger has that itsm has a use for (not release/signing),
   and with every gate that can run on a private repo live *before* the flip.
2. The **workflow gates live once**, in `redscaresu/github-actions`, as a single reusable
   workflow a consumer adopts with one job and one pinned ref. Fixing a pin or a workaround there
   reaches every consumer through one Dependabot PR each. Per-repo config files are copies and
   are allowed to drift (Design §1).
3. The shared repo is **versioned**: semver tags, consumers pin a commit SHA with the tag in a
   comment, so a consumer can see at a glance what it runs and Dependabot can bump it. *tag-v1-0-0 (https://github.com/redscaresu/itsm/pull/36): v1.0.0 exists; Dependabot handling of the tagged pin unverified until its first bump*
4. The shared repo **dogfoods itself on real code**: it carries a minimal Go fixture so every job
   in the bundle runs for real on its own PRs, and its own zizmor audit covers its own workflows.
   A tag is cut only from a green dogfood run.
5. Adopting a new repo is "one job plus six copied files" (Design §1 lists them).

## Non-goals

- Release tooling (binary builds, signing, Homebrew tap). itsm ships no binaries; goldfinger's
  `release.yml` stays where it is.
- Migrating goldfinger or any repo other than itsm onto the shared bundle. Designed for, not
  done here.
- Rewriting git history. The scan found nothing to remove.
- A drift check for the copied templates. Accepted as manual (Design §1).
- Owner-only repo settings (branch protection, secret scanning, dependency graph, code
  scanning). Listed in Rollout as the user's steps; nothing in CI can set them. *go-public: done by the lead under the owner's 2026-10-11 delegation*
- Composite actions. Everything here is a whole job, not a step inside someone else's job; a
  reusable workflow is the right unit.
- A reusable `go-ci.yml` for vet/test/lint. Offered and declined: it is where project-specific
  flags accumulate, and itsm's e2e job could not move anyway (needs Chromium).

## Current state

**Pre-publication secret check (done 2026-10-09, read-only).** `gitleaks detect --log-opts=--all`
over all 14 commits: *no leaks found*. A manual grep of `git log --all -p` for key-shaped strings
finds only the deliberate test value `TYPESAFE_API_KEY=e2e-fake-key`
(`internal/demo/walkthrough_e2e_test.go:97`); the remaining high-entropy strings are `go.sum`
module hashes. The key never appears in `testdata/demo/tickets.json`. History is clean; no rewrite
is needed before going public. Caveat: gitleaks' default ruleset is provider patterns plus a
generic high-entropy rule; the TypeSafe key's shape is not known to the owner, so this scan shows
the scanner runs, not that it would catch this specific key (Risks). *Pre-flip re-scan (go-public, 2026-10-11): fresh git clone --mirror, gitleaks --log-opts='--all -m', 92 commits, 40 refs incl. 39 refs/pull, 0 leaks*

**What CI does today** (`.github/workflows/ci.yml`, last run 37602917492 green, ~1m46s):

| Job | Steps | Notes |
|---|---|---|
| `check` | `go vet`, `go vet -tags e2e,live`, `go test -race`, golangci-lint v2.14.0 | mirrors `make check` (`Makefile:24`) |
| `e2e` | install Playwright driver + Chromium, `go test -tags e2e ./internal/demo/...` | mirrors `make e2e`; needs Chromium, so its own job |

Already right: workflow-level `permissions: contents: read` (`ci.yml:8`); lint config pinned to a
version; `go-version-file: go.mod`; e2e replays recorded jev answers so CI needs no API key and
no repository secret exists. Missing versus goldfinger, from its `.github/` at `6521d54`:

| goldfinger has | itsm today |
|---|---|
| Actions pinned by SHA with `# vX.Y.Z` comment, `persist-credentials: false` on every checkout | floating tags, default credential persistence *done: pin-ci-actions, https://github.com/redscaresu/itsm/pull/27* |
| `gitleaks` job (full history, `--redact`, `.gitleaks.toml` allowlisting `go.sum`) + `make hooks` pre-commit hook (`scripts/pre-commit`) | none *done: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32* |
| `govulncheck` job (`go run ...@latest`) | none *done: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32* |
| `dependabot.yml`: gomod + github-actions, weekly, grouped, no auto-merge | none *done: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32* |
| `zizmor.yml`: blocking audit of `.github/workflows` (`pipx run --spec zizmor==1.29.0`) | none (local zizmor: 0 findings after pin-ci-actions, https://github.com/redscaresu/itsm/pull/27) *done: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32* |
| `codeql.yml`: Go, push/PR/weekly, `security-events: write` | none |
| `dependency-review.yml`: fails PRs adding a vulnerable dep; skips green when the dependency graph is off | none *notice path while private: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32* |
| `scorecard.yml`: OpenSSF Scorecard, publishes score, weekly | none |
| `.golangci.yml` with `gosec` + `staticcheck`, `max-issues-per-linter: 0`, `max-same-issues: 0` | `standard` set + bodyclose, errorlint, misspell, noctx, revive, unconvert; no gosec; golangci default caps (50 / 3) |
| `SECURITY.md` (private reporting, threat model, audit map) | none *done: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32* |
| `release.yml` (binaries, signing) | n/a: itsm ships no binaries |

**gosec on itsm today** (reviewer, gosec v2.29.0, `-tags e2e,live`, production files only): two
findings. G304 at `internal/demodata/demodata.go:80` (`os.ReadFile` of a path every caller
sets to the constant demodata.File or a test fixture) and G710 open redirect at
`internal/server/handlers_customer.go:263`. The second is a false positive: the target is
`ticketPath(id)` = `"/tickets/" + id` (`handlers_customer.go:131`), which cannot start with `//`.
Both get a trailing `// #nosec CODE -- reason`. Nothing in `cmd/`. *Done by gosec-lint
(https://github.com/redscaresu/itsm/pull/24): gosec runs in the lint job and requires a reason on
every `#nosec`. golangci-lint v2.14.0's gosec ignores bare `//nosec`; only `#nosec` suppresses.*

**`redscaresu/github-actions` today:** one commit (`99bd0b1 Initial commit`), `LICENSE`
(Apache-2.0) and a two-line `README.md`. No workflows, no Go code. Public, which is required for
public consumers: a public repo cannot call a private repo's reusable workflow.

**Reusable-workflow facts that constrain the design** (GitHub docs "Reusing workflows",
"About code scanning", "Troubleshooting code scanning"; scorecard-action README):

- A caller job is `uses: redscaresu/github-actions/.github/workflows/<name>.yml@<ref>`; the ref
  must itself be SHA-pinned for zizmor/Scorecard's Pinned-Dependencies, and Dependabot's
  `github-actions` ecosystem bumps reusable-workflow refs like any action.
- The called workflow cannot widen `permissions:`; the caller job must grant what the gate needs
  (`security-events: write` for CodeQL's SARIF upload). Jobs inside the called workflow can
  narrow it back.
- A caller's workflow-level `env:` does **not** reach the called workflow. The bundle sets no
  `env:` at all: every pinned JS action already declares node24 (bundle-gates, survey fact 11).
- Triggers stay in the caller. The called workflow sees the caller's `github.event`, so a
  `dependency-review` job works when the caller runs it on `pull_request`.
- **Code scanning (CodeQL SARIF upload, Scorecard's `upload-sarif`) needs a public repo or a
  Code Security licence.** On private free-plan itsm the `codeql` job and `scorecard.yml` fail.
  This decides the rollout order.
- **Fork PRs can upload SARIF.** "Code scanning always allows the uploading of results when the
  `pull_request` event triggers the action run" (Troubleshooting code scanning, "Resource not
  accessible by integration"). So `codeql` can gate every PR from the first public one.
- **Scorecard publishing from a reusable workflow is believed unsupported.** The OpenSSF API
  verifies the workflow file in the *publishing* repo and requires the scorecard job's steps to
  be only approved actions (README "Workflow Restrictions"); a caller job that `uses:` another
  workflow has no steps. The README does not name `workflow_call` explicitly, so this is an
  inference: **verify by probe before v1.0.0**. If wrong, Scorecard moves into the bundle.
  *Probe answer (2026-10-10): REFUTED.* Scorecard published from a job inside an
  `on: workflow_call` workflow (https://github.com/redscaresu/github-actions/actions/runs/38005404874;
  both API hosts return `repo.commit` = the merge SHA, score 4.1). The owner confirmed
  decision 6 on 2026-10-10: Scorecard is in the bundle as its own reusable `scorecard.yml`.
- `dependabot.yml`, `.gitleaks.toml`, `.golangci.yml` and `SECURITY.md` are per-repo files, not
  workflows; they can only be templated, not called.

**Repo settings that only the owner can change** (the API refuses them on a private free-plan
repo: branch protection and rulesets → HTTP 403 "Upgrade to GitHub Pro or make this repository
public"): branch protection / rulesets on `main`, Dependabot alerts, dependency graph, secret
scanning + push protection, private vulnerability reporting, code scanning and its Check Failure
severity. All become available at no cost once the repo is public. *done: go-public, 2026-10-11*

## Design

### 1. `redscaresu/github-actions`: one reusable workflow, `security.yml`

`.github/workflows/security.yml`, `on: workflow_call`. One job per gate, each with its own
job-level `permissions:` (least privilege is kept inside the bundle even though the caller must
grant the union) and an `if:` that selects the events the gate is meaningful on:

| Job | Runs on | Permissions | Go-only | Source in goldfinger |
|---|---|---|---|---|
| `zizmor` | push, PR | `contents: read` | no | `zizmor.yml` |
| `gitleaks` | push, PR, schedule | `contents: read`, `fetch-depth: 0`, `--redact` | no | `ci.yml` gitleaks job + `.gitleaks.toml` |
| `govulncheck` | push, PR, schedule | `contents: read` | yes, step-level `if: hashFiles('go.mod') != ''` (not available in a job `if:`) | `ci.yml` govulncheck job |
| `codeql` | push, PR, schedule | `security-events: write` | language from input, default `go` | `codeql.yml` |
| `dependency-review` | PR only | `contents: read` | no | `dependency-review.yml`, guard changed (below) |

Rules for the bundle:

- **Every binary a job runs is either an action pinned by SHA or a download verified against a
  pinned checksum.** goldfinger's `pipx run --spec zizmor==x`, unchecksummed `curl` of the
  gitleaks tarball and `govulncheck@latest` are not carried over. Candidates: the
  `zizmorcore/zizmor-action` and `gitleaks/gitleaks-action` actions (SHA-pinned; gitleaks-action
  is free for user-owned repos), govulncheck at a pinned version (reproducible through the Go
  checksum DB; the vulnerability DB is fetched live by design). Chosen per job in the story.
- **gitleaks redacts** (`--redact`): on a public repo Actions logs are public, and an unredacted
  finding republishes the secret.
- **dependency-review fails, not skips, on a public repo** when the dependency graph is off.
  goldfinger's guard exits green with a notice on the "graph disabled" 403; that is fail-open for
  exactly the misconfiguration the gate exists to catch. The graph is on by default for public
  repos, so on `github.event.repository.private == false` the skip path becomes an error; private
  consumers keep the notice. Transient errors still fail closed.
- **The bundle has no `env:` or `defaults:` at any level**: every pinned JS action declares node24,
  and Scorecard refuses to publish from a workflow with workflow-level `env:` or `defaults:`.
- **No `pull_request_target`, no `workflow_run`** in the bundle or in any consumer caller; zizmor's
  `dangerous-triggers` audit enforces it once zizmor is a required check.
- Inputs are the few knobs a consumer actually needs: `codeql-languages` (default `go`),
  `dependency-review-fail-on` (default `moderate`), and one boolean per gate to opt out (`codeql`
  is the one itsm actually uses, while private). No input for anything a second consumer has not
  asked for.

Not in the bundle, shipped as `templates/` to copy in (six files, five after decision 6 removed `scorecard.yml`; drift is accepted and manual,
each template carries a `# template: <name> @vX.Y.Z` header so a consumer can see what it has; `<name>` is the templates/ filename, the hooks.mk header is not carried (decision 4)):

- **`scorecard.yml`**: cannot publish from a reusable workflow (Current state). The one per-repo
  workflow. *Removed by decision 6 (probe REFUTED): five templates; Scorecard is the bundle's
  reusable `.github/workflows/scorecard.yml`, called from its own per-repo caller file.*
- **`dependabot.yml`**: gomod + github-actions, weekly, grouped, no auto-merge, cooldown default-days 7 (zizmor dependabot-cooldown). The
  github-actions ecosystem entry is what keeps the bundle's own ref fresh.
- **`gitleaks.toml`**: default ruleset plus the `go.sum` allowlist. No TypeSafe-specific rule:
  the key's shape is not known (Open questions), so the generic entropy rule is what runs.
- **`SECURITY.md`**: goldfinger's structure, with the rule "a key seen on any public branch is
  rotated, never only removed".
- **`pre-commit`** (gitleaks hook) and the **`hooks` Makefile target** that installs it.

**gosec** is a linter, not a gate of its own; it goes into each consumer's `.golangci.yml`
(goldfinger's `gosec` block, `max-issues-per-linter: 0`, `max-same-issues: 0`) and runs in the
consumer's existing lint job.

### 2. Versioning

- Semver tags on `redscaresu/github-actions`: `vMAJOR.MINOR.PATCH`. MAJOR when a consumer must
  change its caller (new required permission, renamed input, dropped gate); MINOR for a new gate
  or input with a safe default; PATCH for pin bumps and fixes.
- Consumers pin the **commit SHA** with the tag in a trailing comment, exactly as actions are
  pinned: `uses: redscaresu/github-actions/.github/workflows/security.yml@<sha> # v1.0.0`.
  zizmor and Scorecard's Pinned-Dependencies accept this; a floating `@v1` would fail both.
  Dependabot's github-actions ecosystem rewrites SHA and comment together.
- No moving major tag (`v1`). Nobody can safely consume it under SHA pinning, so it would only
  be a second thing to keep in sync.
- A tag is cut by hand with `git tag -a` after the dogfood run on `main` is green. No release
  workflow: there is nothing to build.

### 3. Dogfooding

The shared repo has no code of its own, so a bare dogfood would prove only YAML syntax and
zizmor. It therefore carries a **minimal Go fixture** (`fixture/go.mod`, `fixture/main.go`, a
dozen lines) so `govulncheck`, `codeql` and `dependency-review` have something real to run on,
and `.github/workflows/ci.yml` calls `./.github/workflows/security.yml` by local path (so a PR
tests the uncommitted change, not the last tag) plus copies of the templates *(github-actions dogfoods three: .gitleaks.toml, SECURITY.md, .github/dependabot.yml; no Makefile, so pre-commit and hooks.mk are templates only)*. Its own zizmor job
audits its own workflows, so an unpinned action in the bundle fails the bundle's own PR. What the
dogfood cannot prove: that a *consumer's* code passes the Go gates; itsm's adoption PR is that
test, and v1.0.0 is tagged only after both are green (Rollout).

### 4. itsm as the first consumer

- Add `.github/workflows/security.yml`: triggers `push: main`, `pull_request`, weekly
  `schedule`; one job named `security` that `uses:` the bundle at a pinned SHA with
  `permissions: { contents: read, security-events: write }`. The job name fixes the check names:
  `security / zizmor`, `security / gitleaks`, `security / govulncheck`, `security / codeql`,
  `security / dependency-review`.
- Copy in the six templates. `SECURITY.md` keeps goldfinger's structure: private reporting
  first (GitHub private vulnerability reporting, then the owner's email with a
  `[security] itsm:` subject prefix), then "Handling of credentials" citing
  `cmd/itsm/main.go:95`, `internal/jev/client.go:130`, `internal/server/server.go:17` and README
  "Key handling", and the rotate-on-sight rule.
- Harden the existing `ci.yml` in place: SHA-pin the five action refs, `persist-credentials:
  false` on both checkouts. zizmor goes to zero findings at the default persona (the pedantic
  persona's `anonymous-definition` and `concurrency-limits` are not targeted). `-mod=readonly`
  was considered and dropped: a no-op since Go 1.16. If a lockfile-tidiness gate is wanted later
  it is `go mod tidy && git diff --exit-code go.mod go.sum`, in both CI and `make check`.
- Add `gosec` to `.golangci.yml` with goldfinger's uncapped-issues settings. The existing
  `build-tags: e2e, live` (`.golangci.yml:3-6`) means the e2e harness is linted too, so the
  `_test.go` exclusion is what keeps gosec's file/subprocess noise out of it; production code is
  triaged individually. Today that is the two findings in Current state.
- Add `make hooks` and document it in README "Getting started" (gitleaks must be installed
  locally).

### 5. Enforcement

Every gate is a **required status check by exact name** (Rollout 4): `check`, `e2e`, the five
`security / *` names, and `scorecard` once it exists. Under GitHub's rules a job skipped by an
`if:` reports success, so a Go-only gate on a non-Go repo passes, while a job *missing* from the
PR's workflow file leaves the check "expected" and the PR blocked. That is what stops a PR from
deleting or redirecting its own gate.

What a PR *can* still do is suppress: `// #nosec CODE -- reason`, `//nolint` or `//gosec:disable` for gosec, a `.gitleaks.toml`
allowlist entry, `# zizmor: ignore[rule]`, a `codeql` opt-out input. govulncheck has no
suppression; an upstream fix or a dependency bump is the only exit. GitHub's "require approvals"
cannot be used with one maintainer (you cannot approve your own PR), so the review rule is written
down instead: **the owner reads every diff touching `.github/`, `.gitleaks.toml`, `.golangci.yml`
or adding `#nosec` / `//nolint` / `//gosec:disable` / `zizmor: ignore` before merging**, and a fork PR that touches them is not
merged without that read. CODEOWNERS is not used: it only bites with required review. *Delegated 2026-10-11: the owner gave these reads to the lead until this HLD closes (go-public); the rule stays the owner's standing rule and SECURITY.md is unchanged.*

Which runs gate and which detect: on `pull_request` every job blocks the merge. CodeQL blocks
only for alerts at or above the repository's **Check Failure** severity (default `error`,
`critical`, `high`; an owner setting, Rollout 4). On `push` to `main` and on `schedule` nothing
blocks; those runs file alerts (CodeQL, Dependabot) or fail a workflow the owner sees
(govulncheck, gitleaks). "Advisory first" was rejected: a non-blocking security check gets
ignored from day one, and itsm's dependency surface is five modules, so noise is unlikely.

Pre-exposure versus post-exposure, for the key: the gitleaks pre-commit hook (bypassable with
`--no-verify`) and GitHub push protection (partner patterns only on the free plan) act before a
push. CI gitleaks, the history scan and secret scanning act after it, when the key is already
public. The response to any post-exposure hit is rotation (SECURITY.md).

## Alternatives

- **Copy goldfinger's `.github/` into itsm.** The original brief. The honest trade: action pins
  in copies do not drift (Dependabot), structure and workarounds do. Rejected because the owner
  wants the same posture on ~30 repos and accepts the shared repo on the critical path.
- **One reusable workflow per gate.** Considered first. Same per-job permissions and
  required-check granularity as the bundle, but six refs to pin and bump per consumer. Rejected
  for adoption cost; the bundle's only real downside is that one gate's breaking change bumps
  the ref for all consumers, which is acceptable for a single owner.
- **CodeQL default setup instead of a `codeql` job.** A settings click per repo: no workflow, no
  `security-events: write` in the caller (the bundle would be read-only everywhere), queries kept
  current by GitHub, same Scorecard SAST credit. Rejected: not pinnable, not versioned, one click
  per repo across 30, and enabling it "disables the existing workflow file and blocks CodeQL
  analysis API uploads", so a stray click silently turns the gate off. The bundle job buys a
  pinned, reviewable, zero-click gate.
- **Composite actions.** A step, not a job: cannot set permissions, cannot have its own
  `runs-on`, and would need the consumer to write the job around it. Rejected.
- **A moving `v1` tag.** Rejected: incompatible with SHA pinning, which both zizmor and
  Scorecard require.
- **Scorecard inside the bundle with `publish_results: false`.** Works, but loses the badge and
  the public score, which is the whole point of Scorecard for an adopter. Template instead,
  pending the probe in Current state. *Probe REFUTED; decision 6 puts it in the bundle with
  `publish_results: true`.*
- **A reusable `go-ci.yml`.** Declined (Non-goals).
- **Go public first, bundle after.** Fastest to public and takes the shared repo off the critical
  path. Rejected by the user: the gates that can run while private should be live before the flip.

## Risks and safety

- **The shared repo is a supply-chain hop, twice.** Every consumer runs whatever is at the
  pinned SHA, and every bundle job runs the tools it fetches. Mitigations: SHA pins on the ref (a
  tag move changes nothing); the "pinned action or checksummed download" rule in Design §1; the
  shared repo runs its own bundle and zizmor on every PR; the owner reviews every Dependabot bump
  there. Branch protection on `redscaresu/github-actions` `main` is an owner step (Rollout).
- **Caller permission union.** A consumer grants `security-events: write` to the whole bundle
  job. Job-level `permissions:` inside `security.yml` narrow it back for every job except
  `codeql`, so the write scope reaches exactly one job. Verified by zizmor's
  `excessive-permissions` audit running on the bundle itself.
- **The gitleaks gate is unverified against the one secret it guards.** The default ruleset has
  no TypeSafe pattern and the key's shape is unknown, so detection rests on the generic entropy
  rule. Accepted for now; if the owner learns the format, a custom rule with a fixture that
  proves it fires is a one-story change to the template. The real control is still README "Key
  handling" (Keychain, per-shell, never in an rc file), and rotation on any sighting.
- **Cost.** All public repos: Actions minutes are free. CodeQL adds roughly 3 to 5 minutes per
  push on a repo of this size; the weekly schedules add one run each. No paid features.
- **The e2e job's downloads are unverified.** `go run .../playwright install --with-deps
  chromium` (`ci.yml:46`) fetches the Playwright driver and Chromium with no checksum, every run.
  No gate here covers it. Accepted: the driver version comes from `go.mod` (govulncheck covers
  the Go side), Chromium comes from Playwright's CDN, and the job runs with a read-only token and
  no secrets, so a compromise reaches nothing but the runner.
- **No production exposure.** Nothing here touches the running service, the TypeSafe key, or any
  cloud; CI needs no secret because e2e replays recorded answers.
- **Nothing is an ADR concern.** The repo has no ADRs; the one standing rule this touches is
  README "Key handling", which `SECURITY.md` will restate rather than change.

## Rollout

Each step ships on its own and leaves both repos green, given the order below.

1. **`redscaresu/github-actions`: the bundle, untagged.** `security.yml` with the five jobs, plus the reusable `scorecard.yml` (decision 6), the
   Go fixture, `templates/` (six files; five after decision 6), the dogfood `ci.yml`, a README that shows the one-job
   caller. Probe the Scorecard-in-reusable question while here. Merge to `main` on a green
   dogfood; no tag yet.
   *Owner step:* branch protection on `main` requiring the dogfood checks.
2. **itsm: harden `ci.yml` in place.** SHA pins, `persist-credentials: false`. No behaviour
   change; zizmor at the default persona goes to zero. Independent of step 1.
3. **itsm: adopt the bundle, still private.** Caller `security.yml` pinned to the step-1 SHA with
   `codeql: false`; copy in five templates (not `scorecard.yml`); `SECURITY.md` adapted to itsm's
   key handling; `gosec` in `.golangci.yml` with the two findings triaged; `make hooks` and the
   README line. Green on private itsm proves the Go gates on a real consumer. *done: adopt-bundle-private, https://github.com/redscaresu/itsm/pull/32; dependency-review notice path only*
4. **`redscaresu/github-actions` v1.0.0.** Tag the step-1 SHA once step 3 is green; itsm's
   caller comment becomes `# v1.0.0` in its next Dependabot bump or by hand. *done: tag-v1-0-0, https://github.com/redscaresu/itsm/pull/36; annotated tag on the adoption pin X, not the step-1 SHA; comment by a hand PR; Dependabot handling of the tagged pin unverified until its first bump*
5. **itsm: go public.** *Owner steps, in this order:* add `LICENSE` (Apache-2.0, decided); flip
   visibility; Settings → Code security: Dependabot alerts + security updates, dependency graph,
   secret scanning + push protection, private vulnerability reporting, code scanning **without**
   clicking "default setup", Check Failure severity left at the default; Actions → confirm
   "Require approval for first-time contributors"; ruleset on `main` requiring `check`, `e2e`
   and the five `security / *` checks. *done: go-public, https://github.com/redscaresu/itsm/pull/39; by the lead under the owner's 2026-10-11 delegation; fork-PR approval was a change from the public default (not read before the PUT; GitHub documents first_time_contributors_new_to_github), not a confirm; Dependabot security updates on after the ruleset; Check Failure severity untouched, no API reads it*
6. **itsm: the public-only gates.** Flip `codeql: false` off in the caller, copy in
   `scorecard.yml` (*decision 6: a `.github/workflows/scorecard.yml` caller of the bundle's reusable `scorecard.yml` instead; it alone grants `id-token: write`*),
   add `security / codeql` and `scorecard` to the required checks. The Scorecard
   badge appears after its first scheduled run.
7. **Later, not here:** goldfinger moves its gates onto the bundle and deletes its copies; other
   repos adopt with "one job plus six files".

## Open questions

- Pinned versions for the bundle's first tag (gitleaks or gitleaks-action, zizmor or
  zizmor-action, govulncheck, codeql-action, scorecard-action, dependency-review-action,
  checkout, setup-go): resolved at build time to the current releases, not decided here.
- TypeSafe key format. Unknown to the owner as of 2026-10-09; the generic entropy rule is relied
  on and the gap is recorded in Risks. Revisit if the format becomes known.
- Local `check:` for stories in the `github-actions` repo (it has no Makefile): proposed
  `zizmor --offline .github/` plus `go vet ./fixture/...`; whether actionlint joins it, and the
  zizmor version (local 1.30.1 versus a pin), are decided in the first story there.
- Whether Scorecard really cannot publish from a reusable workflow (probe in Rollout 1).
  *Answered 2026-10-10: it can (REFUTED, https://github.com/redscaresu/github-actions/actions/runs/38005404874); decision 6 confirmed.*

## Epics

In rollout order. Decomposed 2026-10-09; the owner approved the epics and the decisions below.

- bundle-scorecard-probe (done: REFUTED, Scorecard is a bundle job)
- pin-ci-actions (done 2026-10-11: https://github.com/redscaresu/itsm/pull/27)
- gosec-lint (done 2026-10-10: https://github.com/redscaresu/itsm/pull/24)
- bundle-gates (done 2026-10-10: five gates in `security.yml`, Scorecard in reusable `scorecard.yml`, dogfood push run https://github.com/redscaresu/github-actions/actions/runs/38046711288; negative control https://github.com/redscaresu/github-actions/pull/10 closed unmerged; with the dependency graph turned off, `security / dependency-review` failed with the public-repo `::error::`)
- bundle-templates-readme (done 2026-10-11: https://github.com/redscaresu/github-actions/pull/12, https://github.com/redscaresu/github-actions/pull/11)
- adopt-bundle-private (done 2026-10-11: https://github.com/redscaresu/itsm/pull/32)
- tag-v1-0-0 (done 2026-10-11: https://github.com/redscaresu/itsm/pull/36; v1.0.0 = c2c992c0fe67b561b37874af8a429972ad856d69; Dependabot handling of the tagged pin unverified until its first bump)
- go-public (done 2026-10-11: https://github.com/redscaresu/itsm/pull/39; public 2026-10-11T03:26:41Z; mirror scan 0 leaks, 92 commits = rev-list --all, 40 refs incl. 39 refs/pull; secret scanning 0 open alerts; deleted readme-demo-gif, story/gosec-lint-config; branches left: main; ruleset 24875133 requires the seven; throwaway https://github.com/redscaresu/itsm/pull/40 closed unmerged)
- public-only-gates (done 2026-10-11: codeql on https://github.com/redscaresu/itsm/pull/42, security / codeql success, CodeQL analyses on code scanning; Scorecard caller and badge https://github.com/redscaresu/itsm/pull/43, push run on main security / scorecard success, both Scorecard APIs report score 6 at a7d2789; ruleset 24875133 still the seven, scorecard not required)

### Decisions taken at decomposition

These correct the text above where they disagree with it.

1. **Where does the dogfood fixture's go.mod live? HLD:169 gates govulncheck on `hashFiles('go.mod') != ''` (repo root) but HLD:231 puts it at `fixture/go.mod`, and HLD:192-195 forbids a directory input.** `go.mod` at the github-actions root (module `github.com/redscaresu/github-actions`), `fixture/main.go` as its only package; local check `go vet ./...`. No new input. Alternative rejected: a `go-directory` input is a knob no consumer has asked for. (corrects HLD lines 169, 192-195, 231-232, 382)
2. **Should `scorecard` be a required status check?** No. The template runs only on push:main, schedule and branch_protection_rule (goldfinger scorecard.yml:9-14); a required check no PR reports blocks every merge (HLD:267-268). Scorecard is a post-merge detector. Delete 'and `scorecard` once it exists' from Enforcement and 'and `scorecard`' from Rollout 6. (corrects HLD lines 265, 368-369)
3. **Which step adds `security / codeql` to the ruleset?** Rollout 5 (go-public) requires all five `security / *` from the first ruleset; the skipped codeql job reports success until Rollout 6 turns it on. Rollout 6 changes nothing in the ruleset. (corrects HLD lines 366-369)
4. **What do template headers and the caller comment say before v1.0.0 exists, and who rewrites them?** Shared `templates/` carry `template: <name>` with no version (the repo copy cannot know its future tag). Consumer copies carry `template: <name> @<version>` and the caller `# <version>`, both `unreleased` until the tag; tag-v1-0-0's one itsm PR rewrites all five to `v1.0.0`. Later consumers write the tag they copied from. (corrects HLD lines 198-199, 361) *Amended 2026-10-11 (bundle-templates-readme): the consumer caller line carries no version comment until tag-v1-0-0 (zizmor's online ref-version-mismatch flags `# unreleased`, and `# v1.0.0` before the tag exists; checked 2026-10-11); template headers keep `@unreleased`.* *done: tag-v1-0-0, https://github.com/redscaresu/itsm/pull/36: caller `# v1.0.0`, the four headers `@v1.0.0` (hooks.mk carries none)*
5. **'Six copied files' names five files plus a Makefile target, and Rollout 3 copies five. Which is it?** Keep six: the sixth is `templates/hooks.mk` (the `hooks` target), applied as a Makefile edit in a consumer that has one. Rollout 3 copies dependabot.yml, gitleaks.toml, SECURITY.md, pre-commit and applies hooks.mk; scorecard.yml waits for Rollout 6. Decision 6 later removed `templates/scorecard.yml`, leaving five. (corrects HLD lines 60-61, 197-208, 246, 358-359)
6. **If the probe shows Scorecard can publish from a reusable workflow, what changes and who owns it?** Follow HLD:147: bundle-gates adds a separate reusable `.github/workflows/scorecard.yml` (so `security.yml` callers never grant `id-token: write`); `templates/scorecard.yml` is dropped; README shows the consumer's `.github/workflows/scorecard.yml` caller: `on:` push to main and a weekly schedule only, `permissions: {}`, one job `security` (`name: security`), `uses: redscaresu/github-actions/.github/workflows/scorecard.yml@<sha> # <version>`, `permissions: {contents: read, id-token: write}`; never in a workflow that runs on pull_request; public-only-gates' second story adds that caller (check name `security / scorecard`). The owner confirms in bundle-scorecard-probe's your_part before bundle-gates goes active. Probe answer: REFUTED (https://github.com/redscaresu/github-actions/actions/runs/38005404874); the owner confirmed decision 6 on 2026-10-10, so this path applies. (corrects HLD lines 143-147, 200-201, 310-312, 368-370)
7. **Where is the owner-reads-every-sensitive-diff rule written down (CODEOWNERS excluded)?** A 'Maintainer review' section in `templates/SECURITY.md`, so every consumer carries it; itsm's SECURITY.md gets it at adoption. (corrects HLD lines 278-284)
8. **May the public shared repo hold a never-merged negative-control PR with a synthetic secret and a vulnerable module? Its `refs/pull` ref is permanent and public.** Yes: it is the only proof the gates fire (Goal 4). The value is a documented example format, not a credential, and not a partner pattern (push protection is on in github-actions and would block one); the module is an old public release. Title it 'negative control: do not merge'; the owner closes it. (corrects HLD lines 57-59, 228-237)
9. **Does the README get a Scorecard badge, and when?** Yes, in the scorecard story's PR: the badge resolves after the first publish, which happens on the push:main run right after merge, not a week later. (corrects HLD lines 369-370)
10. **Who adds LICENSE?** An agent PR, the first story of go-public; the owner reads and merges. Only the flip and Settings are owner steps. (corrects HLD lines 362) *done: go-public, https://github.com/redscaresu/itsm/pull/39: merged by the conductor (merge = agent; not a sensitive diff); the flip and Settings by the lead under the owner's 2026-10-11 delegation*
