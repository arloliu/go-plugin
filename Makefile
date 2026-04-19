# Makefile for the go-plugin fork.
#
# Targets mirror .github/workflows/test.yaml so `make check` locally is the
# same gate CI runs. Dev-loop targets (`test`, `lint`, `fmt`) give quick
# feedback without the full CI bundle.

GO              ?= go
GOLANGCI_LINT   ?= golangci-lint
COVERAGE_OUT    ?= coverage.out
PKG             ?= ./...

.DEFAULT_GOAL := help

## help: Print this help.
.PHONY: help
help:
	@awk 'BEGIN{FS=":.*?## "} /^## / {sub(/^## /,""); print}' $(MAKEFILE_LIST) | \
		awk -F: '{printf "  \033[36m%-14s\033[0m %s\n", $$1, substr($$0, index($$0,":")+2)}'

## fmt: Run go fmt; rewrites files in place.
.PHONY: fmt
fmt:
	$(GO) fmt $(PKG)

## fmt-check: Fail if any file is not gofmt-clean (CI gate).
.PHONY: fmt-check
fmt-check:
	@files=$$($(GO) fmt $(PKG)); \
	if [ -n "$$files" ]; then \
		echo "The following file(s) are not gofmt-clean:"; \
		echo "$$files"; \
		exit 1; \
	fi

## vet: Run go vet across all packages.
.PHONY: vet
vet:
	$(GO) vet $(PKG)

## lint: Run golangci-lint (same action CI uses).
.PHONY: lint
lint:
	$(GOLANGCI_LINT) run $(PKG)

## test: Run the full test suite with the race detector.
.PHONY: test
test:
	$(GO) test -race $(PKG)

## test-short: Quick feedback — tests without -race, for iteration.
.PHONY: test-short
test-short:
	$(GO) test $(PKG)

## cover: Run tests with coverage (mirrors CI, including -race).
.PHONY: cover
cover:
	$(GO) test -race -coverprofile=$(COVERAGE_OUT) $(PKG)
	$(GO) tool cover -func=$(COVERAGE_OUT) | tail -1

## cover-html: Open the coverage report in a browser.
.PHONY: cover-html
cover-html: cover
	$(GO) tool cover -html=$(COVERAGE_OUT)

## build: Compile all packages.
.PHONY: build
build:
	$(GO) build $(PKG)

## check: Run the full CI gate locally (fmt-check + vet + lint + test + build).
.PHONY: check
check: fmt-check vet lint test build

## testdata: Rebuild cross-platform test binaries under internal/cmdrunner/testdata.
.PHONY: testdata
testdata:
	$(MAKE) -C internal/cmdrunner/testdata

## clean: Remove coverage artifacts.
.PHONY: clean
clean:
	rm -f $(COVERAGE_OUT)

## update-pkg-cache: Attempt to update the Go module proxy cache for the latest git tag. Not critical, so failure is non-fatal.
.PHONY: update-pkg-cache
update-pkg-cache:
	@echo "Updating package cache with latest git tag: $(LATEST_GIT_TAG)"
	@curl -sf https://proxy.golang.org/github.com/arloliu/go-plugin/@v/$(LATEST_GIT_TAG).info > /dev/null || \
		echo "Warning: Failed to update package cache"
