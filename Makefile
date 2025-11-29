.PHONY: help test test-unit test-corpus test-all lint build clean fmt vet tidy examples

LINTER = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.2"

# Default target
help:
	@echo "Available targets:"
	@echo "  make test         - Run unit tests"
	@echo "  make test-corpus  - Run fuzz corpus regression tests"
	@echo "  make test-all     - Run all tests (unit + corpus)"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make fmt          - Format code with gofmt"
	@echo "  make vet          - Run go vet"
	@echo "  make tidy         - Run go mod tidy (main + test)"
	@echo "  make build        - Build the package"
	@echo "  make examples     - Build example to ./bin/"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make ci           - Run all CI checks (fmt, vet, lint, test-all)"

# Go environment
GO := go

# Run unit tests
test: test-unit

test-unit:
	@cd test && $(GO) test -v -race -coverprofile=./coverage.txt -covermode=atomic ./... || exit 1

# Run fuzz corpus regression tests
test-corpus:
	@cd test && $(GO) test -v -run=TestFuzzCorpus || exit 1

# Run all tests
test-all: test-unit test-corpus

# Run linter
lint:
	go run $(LINTER) run ./... --timeout=5m

# Format code
fmt:
	gofmt -s -w .

# Run go vet
vet:
	$(GO) vet ./...

# Run go mod tidy
tidy:
	@echo "Running go mod tidy for main package..."
	@$(GO) mod tidy || exit 1
	@echo "Running go mod tidy for test..."
	@cd test && $(GO) mod tidy || exit 1

# Build the package
build:
	$(GO) build ./...

# Build examples to ./bin/
examples:
	@mkdir -p bin
	@for example in examples/*/; do \
		name=$$(basename $$example); \
		echo "Building $$name..."; \
		cd $$example && \
		$(GO) mod tidy && \
		$(GO) build -o ../../bin/$$name . || exit 1; \
		cd ../..; \
	done
	@echo "Examples built to ./bin/"

# Clean build artifacts
clean:
	$(GO) clean
	rm -f coverage.txt
	rm -rf bin
	cd test && $(GO) clean

# Run all CI checks locally
ci: fmt vet lint test-all
	@echo "All CI checks passed!"
