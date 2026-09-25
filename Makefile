GO ?= go

.PHONY: build run test vet lint check

build:
	$(GO) build -o bin/itsm ./cmd/itsm

# Needs TYPESAFE_API_KEY in the shell (see README); never put it in this file.
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
