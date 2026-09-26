GO ?= go

.PHONY: build run test vet lint check demo demo-install

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

# Everything CI runs.
check: vet test lint

# Once per machine: downloads the Playwright driver (its own Node.js runtime,
# not npm) and Chromium for the walkthrough. Allow a few minutes.
demo-install:
	$(GO) run ./cmd/demo install

# The scripted browser walkthrough (HLD 10). Needs TYPESAFE_API_KEY in the
# shell. Starts bin/itsm, plays the ten scenes in two windows, stops it.
demo: build
	$(GO) run ./cmd/demo
