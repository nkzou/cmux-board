BINARY := cmux-board
GO := go
COVERAGE_FILE := coverage.out

.PHONY: build install test lint clean coverage test-integration test-e2e verify-additive

build:
	$(GO) build -o $(BINARY) ./cmd/cmux-board

install:
	$(GO) install ./cmd/cmux-board

test: verify-additive
	$(GO) test -race ./...

test-integration:
	$(GO) test -race -tags integration ./...

coverage:
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) ./...

lint:
	$(GO) vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then staticcheck -checks=all,-U1000,-ST1000,-ST1005 ./...; fi

clean:
	rm -f $(BINARY) $(COVERAGE_FILE) coverage.html

test-e2e:
	$(GO) test -tags e2e -race -timeout 15m ./internal/runtime/...

verify-additive:
	bash scripts/check_additive_only.sh
