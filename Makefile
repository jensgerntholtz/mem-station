# MemStation development tasks.
#
# One entry point for local runs and CI, so the Charm v2 migration has a single
# command to verify against. Nothing here needs root, hardware, dmidecode,
# decode-dimms, or stress-ng; the integration target launches /bin/sh only.
#
#   make            # check: gofmt, vet, build, test
#   make race       # the same suite with the race detector
#   make goldens    # rewrite golden views (review the diff before committing)
#   make run        # build and launch the TUI

GO ?= go
BIN := mem-station
GOLDEN_PKGS := ./internal/app ./internal/benchmark ./internal/burnin

.DEFAULT_GOAL := check

.PHONY: check gofmt vet build test race integration goldens goldens-check run bin tidy clean help

## check: format, vet, build and test (what CI runs)
check: gofmt vet build test

## gofmt: fail if any file is not gofmt-clean
gofmt:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt is required for:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## vet: run go vet on the module
vet:
	$(GO) vet ./...

## build: compile every package
build:
	$(GO) build ./...

## test: run the default suite (no root, no hardware, no stress-ng)
test:
	$(GO) test ./... -count=1

## race: run the suite under the race detector
race:
	$(GO) test ./... -race -count=1

## integration: compile and run the tagged runner tests (launches /bin/sh)
integration:
	$(GO) vet -tags integration ./internal/runner
	$(GO) test -tags integration ./internal/runner -count=1

## goldens: rewrite golden views, then print the diff summary
goldens:
	MEM_STATION_UPDATE_GOLDEN=1 $(GO) test ./... -run TestGolden -count=1
	@git diff --stat -- '*/testdata/*.golden' || true

## goldens-check: compare golden views without rewriting anything
goldens-check:
	$(GO) test $(GOLDEN_PKGS) -run TestGolden -count=1

## run: build and launch the TUI
run:
	$(GO) run ./cmd/mem-station

## bin: build ./mem-station only
bin:
	$(GO) build -o $(BIN) ./cmd/mem-station

## tidy: tidy and verify the module graph
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## clean: remove build outputs
clean:
	rm -f $(BIN)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
