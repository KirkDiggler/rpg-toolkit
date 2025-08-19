.PHONY: test lint fmt coverage clean pre-commit help install-tools install-hooks test-all lint-all fmt-all check-versions tag-module release-module

# Default target
help:
	@echo "Available targets:"
	@echo "  test         - Run tests for core and events modules"
	@echo "  test-all     - Run tests for all modules"
	@echo "  lint         - Run golangci-lint on core and events modules"
	@echo "  lint-all     - Run golangci-lint on all modules"
	@echo "  fmt          - Format core and events modules"
	@echo "  fmt-all      - Format all Go code"
	@echo "  coverage     - Generate test coverage report for core and events"
	@echo "  clean        - Remove generated files"
	@echo "  generate     - Run go generate on all modules"
	@echo "  pre-commit   - Run all pre-commit checks (fmt, lint, test)"
	@echo "  fix          - Run all auto-fix commands (fmt, mod-tidy)"
	@echo "  install-tools - Install required development tools"
	@echo "  install-hooks - Install git hooks"
	@echo "  check-versions - Check current versions of all modules"
	@echo "  tag-module   - Tag a specific module with a new version"
	@echo "  release-module - Create a release for a module (runs tests first)"
	@echo "  help         - Show this help message"

# Run tests for all modules
test:
	@echo "Running tests..."
	@echo "→ Testing core module..."
	cd core && go test -race ./...
	@echo "→ Testing events module..."
	cd events && go test -race ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.2.1)
	@echo "→ Linting core module..."
	cd core && golangci-lint run ./...
	@echo "→ Linting events module..."
	cd events && golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go work sync 2>/dev/null || true
	@echo "→ Running gofmt with simplify..."
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./mock/*" -exec gofmt -s -w {} \;
	@echo "→ Running goimports..."
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./mock/*" -exec goimports -w -local github.com/KirkDiggler {} \;
	@echo "→ Ensuring newlines at end of files..."
	@find . -name "*.go" -type f -exec sh -c 'tail -c1 {} | read -r _ || echo >> {}' \;
	@echo "✅ Formatting complete"

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

# Generate mocks and other generated code
generate:
	@echo "Generating code..."
	@find . -name "go.mod" -type f -not -path "./vendor/*" | while read -r modfile; do \
		dir=$$(dirname "$$modfile"); \
		echo "→ Generating in $$dir..."; \
		(cd "$$dir" && go generate ./...) || exit 1; \
	done
	@echo "✅ Code generation complete"

# Pre-commit checks
pre-commit:
	@echo "Running pre-commit checks..."
	@echo "→ Formatting code..."
	@go work sync 2>/dev/null || true
	cd core && go fmt ./...
	cd events && go fmt ./...
	@echo "→ Tidying modules..."
	cd core && go mod tidy
	cd events && go mod tidy
	@echo "→ Running linter..."
	cd core && golangci-lint run ./...
	cd events && golangci-lint run ./...
	@echo "→ Running tests with coverage..."
	@echo "  Testing core..."
	cd core && go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "  Checking core coverage..."
	@cd core && coverage=$$(go tool cover -func=coverage.txt | grep total | awk '{print $$3}' | sed 's/%//') && \
		if [ "$$(echo "$$coverage < 60" | bc -l)" = "1" ]; then \
			echo "❌ Core coverage is $$coverage% (must be at least 60%)"; \
			exit 1; \
		else \
			echo "✓ Core coverage is $$coverage%"; \
		fi
	@echo "  Testing events..."
	cd events && go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "  Checking events coverage..."
	@cd events && coverage=$$(go tool cover -func=coverage.txt | grep total | awk '{print $$3}' | sed 's/%//') && \
		if [ "$$(echo "$$coverage < 60" | bc -l)" = "1" ]; then \
			echo "❌ Events coverage is $$coverage% (must be at least 60%)"; \
			exit 1; \
		else \
			echo "✓ Events coverage is $$coverage%"; \
		fi
	@echo "✓ All pre-commit checks passed!"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.2.1
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "✅ Tools installed successfully"

# Install git hooks
install-hooks:
	@echo "Installing git hooks..."
	git config core.hooksPath .githooks
	@echo "✅ Git hooks installed"

# Test all modules (dynamic discovery)
test-all:
	@echo "Running tests for all modules..."
	@find . -name "go.mod" -type f -not -path "./vendor/*" | while read -r modfile; do \
		dir=$$(dirname "$$modfile"); \
		echo "→ Testing $$dir..."; \
		(cd "$$dir" && go test -race ./...) || exit 1; \
	done

# Lint all modules (dynamic discovery)
lint-all:
	@echo "Running linter on all modules..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.2.1)
	@find . -name "go.mod" -type f -not -path "./vendor/*" | while read -r modfile; do \
		dir=$$(dirname "$$modfile"); \
		echo "→ Linting $$dir..."; \
		if [ -f ".golangci.yml" ]; then \
			(cd "$$dir" && golangci-lint run) || exit 1; \
		else \
			(cd "$$dir" && golangci-lint run --no-config) || exit 1; \
		fi; \
	done

# Format all Go code (dynamic discovery)
fmt-all:
	@echo "Formatting all Go code..."
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./mock/*" -exec gofmt -s -w {} \;
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./mock/*" -exec goimports -w -local github.com/KirkDiggler {} \;
	@echo "→ Ensuring newlines at end of files..."
	@find . -name "*.go" -type f -exec sh -c 'tail -c1 {} | read -r _ || echo >> {}' \;
	@echo "✅ All code formatted"

# Run go mod tidy on all modules
mod-tidy:
	@echo "Running go mod tidy on all modules..."
	@find . -name "go.mod" -type f -not -path "./vendor/*" | while read -r modfile; do \
		dir=$$(dirname "$$modfile"); \
		echo "→ Tidying $$dir..."; \
		(cd "$$dir" && go mod tidy) || exit 1; \
	done
	@echo "✅ All modules tidied"

# Fix all auto-fixable issues
fix: fmt-all mod-tidy
	@echo "✅ All auto-fixable issues resolved"
	@echo "Run 'git add -u' to stage the changes"

# Check current versions of all modules
check-versions:
	@./scripts/check-versions.sh

# Tag a specific module with a new version
tag-module:
	@echo "Tagging module..."
	@echo "Usage: make tag-module MODULE=tools/spatial VERSION=v0.2.0"
	@if [ -z "$(MODULE)" ]; then \
		echo "❌ MODULE is required"; \
		echo "Example: make tag-module MODULE=tools/spatial VERSION=v0.2.0"; \
		exit 1; \
	fi
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ VERSION is required"; \
		echo "Example: make tag-module MODULE=tools/spatial VERSION=v0.2.0"; \
		exit 1; \
	fi
	@if [ ! -f "$(MODULE)/go.mod" ]; then \
		echo "❌ Module $(MODULE) does not exist"; \
		exit 1; \
	fi
	@echo "Creating tag $(MODULE)/$(VERSION)..."
	@git tag -a "$(MODULE)/$(VERSION)" -m "Release $(MODULE) $(VERSION)"
	@echo "✅ Tag created. Run 'git push origin $(MODULE)/$(VERSION)' to push"

# Create a release for a module (runs tests first)
release-module:
	@echo "Creating module release..."
	@echo "Usage: make release-module MODULE=tools/spatial VERSION=v0.2.0"
	@if [ -z "$(MODULE)" ]; then \
		echo "❌ MODULE is required"; \
		echo "Example: make release-module MODULE=tools/spatial VERSION=v0.2.0"; \
		exit 1; \
	fi
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ VERSION is required"; \
		echo "Example: make release-module MODULE=tools/spatial VERSION=v0.2.0"; \
		exit 1; \
	fi
	@if [ ! -f "$(MODULE)/go.mod" ]; then \
		echo "❌ Module $(MODULE) does not exist"; \
		exit 1; \
	fi
	@echo "🧪 Testing module $(MODULE)..."
	@cd "$(MODULE)" && go test ./...
	@echo "✅ Tests passed"
	@echo "Creating tag $(MODULE)/$(VERSION)..."
	@git tag -a "$(MODULE)/$(VERSION)" -m "Release $(MODULE) $(VERSION)"
	@echo "📦 Tag created: $(MODULE)/$(VERSION)"
	@echo ""
	@echo "To publish this release:"
	@echo "  1. Push the tag: git push origin $(MODULE)/$(VERSION)"
	@echo "  2. Create a GitHub release from the tag"
	@echo ""
	@echo "Users can then install with:"
	@echo "  go get github.com/KirkDiggler/rpg-toolkit/$(MODULE)@$(MODULE)/$(VERSION)"