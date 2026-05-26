BINARY := cmux-board
GO := go
COVERAGE_FILE := coverage.out

.PHONY: build test lint clean coverage test-integration verify-additive

build:
	$(GO) build -o $(BINARY) ./cmd/cmux-board

test:
	$(GO) test -race ./...

test-integration:
	$(GO) test -race -tags integration ./...

coverage:
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) ./...

lint:
	$(GO) vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then staticcheck -checks=all,-U1000,-ST1000 ./...; fi

clean:
	rm -f $(BINARY) $(COVERAGE_FILE) coverage.html

verify-additive:
	@echo "TODO: implement in M-011 T-084"
