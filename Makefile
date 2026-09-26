GO ?= go

.PHONY: build run test vet lint check demo demo-install e2e

build:
	$(GO) build -o bin/itsm ./cmd/itsm

# Needs TYPESAFE_API_KEY in the shell; never put it in this file.
run:
	$(GO) run ./cmd/itsm

test:
	$(GO) test -race ./...

# The second vet compile-checks the e2e and live tagged files without running them.
vet:
	$(GO) vet ./...
	$(GO) vet -tags e2e,live ./...

lint:
	golangci-lint run

# Everything CI's check job runs. CI also runs the e2e job: 'make e2e'.
check: vet test lint

# The headless walkthrough: real binary + fake jev + Chromium. Needs demo-install.
e2e:
	$(GO) test -tags e2e -count=1 ./internal/demo/...

# Once per machine: downloads the Playwright driver (its own Node.js runtime,
# not npm) and Chromium for the walkthrough. Allow a few minutes.
demo-install:
	$(GO) run ./cmd/demo install

# The scripted browser walkthrough (HLD 10). Needs TYPESAFE_API_KEY in the
# shell. Starts bin/itsm, plays the ten scenes in two windows, stops it.
demo: build
	$(GO) run ./cmd/demo
