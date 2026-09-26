# itsm

A demo IT self-service portal. A customer describes a problem in plain
English. **jev** (TypeSafe's System One model) answers a fixed set of typed
questions about it. Go code turns those answers into a team, a severity and,
when triage can't decide, pre-written follow-up questions.

- Customer screen: <http://localhost:8080/>
- Agent screen: <http://localhost:8080/agent>

Tickets are kept in memory and are lost on restart.

## Run

You need Go 1.26 and a TypeSafe API key.

```sh
typesafe-key          # load the key into this shell only (see below)
make run              # or: go run ./cmd/itsm
```

The server listens on `PORT` and shuts down cleanly on Ctrl-C. It waits up to
10 s for triage runs still in flight.

## Config

Environment only.

| Variable | Default | Meaning |
|---|---|---|
| `TYPESAFE_API_KEY` | none, required | Bearer token for jev. Never logged. |
| `TYPESAFE_BASE_URL` | `https://api.typesafe.ai` | jev's base URL. Tests point it at a fake. |
| `PORT` | `8080` | HTTP port. |

## Key handling

Keep the key in the macOS Keychain. Load it into one shell with a function.
Never `export` it from a shell rc file: every process started from that
shell would inherit it.

Store it once:

```sh
read -rs T; security add-generic-password -U -a "$USER" -s typesafe-api-key -w "$T"; unset T
```

Add this function to your shell rc:

```sh
typesafe-key() { export TYPESAFE_API_KEY=$(security find-generic-password -a "$USER" -s typesafe-api-key -w); }
```

Run `typesafe-key` in the shell where you start the server or the live test.

## Test

```sh
make check            # everything CI runs: vet, test -race, golangci-lint
go test -race ./...   # unit and end-to-end tests; no key, no network
```

`cmd/itsm/main_test.go` starts the real server with a fake jev and walks a
ticket through its whole life: raised, more information requested,
answered, re-routed, reassigned, closed.

### Live smoke test

Sends the six demo tickets, and ticket 4's follow-up answers, to the real
jev API. For each run it checks jev's team, kind and scope choices against the
table in the design doc (HLD 4.5), then the ticket's team, severity, status
and needs-review flag. It does not compare probabilities; it logs them.
Run it by hand. It skips without a key.

```sh
typesafe-key
go test -tags live -count=1 -v ./cmd/itsm/
```

A failure means jev's answers have drifted. The test logs each run's answers.
Fix a threshold (`internal/triage/thresholds.go`) or a question
(`internal/jev/questions.go`), not the test.

## Demo data

`testdata/demo/tickets.json` holds the six demo tickets: what to type, the
expected outcome, and what live jev answered (replayed by the fake).
`internal/demodata` reads it. The live test and the end-to-end tests both use
this one file, so the typed text and the expected outcomes cannot drift apart.
