.PHONY: test lint fmt coverage clean pre-commit help

# Default target
help:
	@echo "Available targets:"
	@echo "  test       - Run all tests with race detector"
	@echo "  lint       - Run golangci-lint on all modules"
	@echo "  fmt        - Format all Go code"
	@echo "  coverage   - Generate test coverage report"
	@echo "  clean      - Remove generated files"
	@echo "  pre-commit - Run all pre-commit checks (fmt, lint, test)"
	@echo "  help       - Show this help message"

# Run tests for all modules
test:
	@echo "Running tests..."
	@echo "→ Testing core module..."
	cd core && go test -race ./...
	@echo "→ Testing events module..."
	cd events && go test -race ./...
	@echo "→ Testing conditions module..."
	cd mechanics/conditions && go test -race ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0)
	@echo "→ Linting core module..."
	cd core && golangci-lint run ./...
	@echo "→ Linting events module..."
	cd events && golangci-lint run ./...
	@echo "→ Linting conditions module..."
	cd mechanics/conditions && golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go work sync 2>/dev/null || true
	@echo "→ Formatting core module..."
	cd core && go fmt ./...
	@echo "→ Formatting events module..."
	cd events && go fmt ./...
	@echo "→ Formatting conditions module..."
	cd mechanics/conditions && go fmt ./...

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	@echo "→ Coverage for core module..."
	cd core && go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	cd core && go tool cover -html=coverage.txt -o coverage.html
	@echo "  Coverage report: core/coverage.html"
	@echo "→ Coverage for events module..."
	cd events && go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	cd events && go tool cover -html=coverage.txt -o coverage.html
	@echo "  Coverage report: events/coverage.html"

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	find . -name "coverage.txt" -delete
	find . -name "coverage.html" -delete
	find . -name "*.test" -delete

# Pre-commit checks
pre-commit:
	@echo "Running pre-commit checks..."
	@echo "→ Formatting code..."
	@go work sync 2>/dev/null || true
	cd core && go fmt ./...
	cd events && go fmt ./...
	cd mechanics/conditions && go fmt ./...
	@echo "→ Tidying modules..."
	cd core && go mod tidy
	cd events && go mod tidy
	cd mechanics/conditions && go mod tidy
	@echo "→ Running linter..."
	cd core && golangci-lint run --no-config ./...
	cd events && golangci-lint run --no-config ./...
	cd mechanics/conditions && golangci-lint run --no-config ./...
	@echo "→ Running tests..."
	cd core && go test -race ./...
	cd events && go test -race ./...
	cd mechanics/conditions && go test -race ./...
	@echo "✓ All pre-commit checks passed!"