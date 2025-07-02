.PHONY: test lint fmt coverage clean help

# Default target
help:
	@echo "Available targets:"
	@echo "  test      - Run all tests with race detector"
	@echo "  lint      - Run golangci-lint on all modules"
	@echo "  fmt       - Format all Go code"
	@echo "  coverage  - Generate test coverage report"
	@echo "  clean     - Remove generated files"
	@echo "  help      - Show this help message"

# Run tests for all modules
test:
	@echo "Running tests..."
	cd core && go test -race ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0)
	cd core && golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go work sync
	cd core && go fmt ./...

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	cd core && go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	cd core && go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: core/coverage.html"

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	find . -name "coverage.txt" -delete
	find . -name "coverage.html" -delete
	find . -name "*.test" -delete