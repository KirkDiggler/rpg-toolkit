# RPG Toolkit Development Guidelines

## Module Development Workflow

**IMPORTANT: NO go.work FILES OR LOCAL REPLACE DIRECTIVES**

This project uses a clean module development approach:

1. **Each module is developed independently**
   - Work on one module at a time
   - Use published dependencies only
   - No replace directives pointing to local paths
   - No go.work files

2. **Dependency Management**
   - Modules reference published versions (e.g., `v0.1.0`)
   - When you need updates from another module:
     - Push the changes to that module first
     - Then `go get -u` in the dependent module
   - During development, Go creates pseudo-versions automatically (e.g., `v0.0.0-20230907052031-37f5183ecf93`)

3. **Why This Approach**
   - Keeps development honest - you work with real APIs
   - Focuses work on one module at a time
   - Avoids local development issues and CI failures
   - Clear dependency tracking
   - No confusion about which version is being used

## Testing Commands

When working on a module:
```bash
# Run tests
go test ./...

# Run linter
golangci-lint run ./...

# Update dependencies
go get -u ./...
go mod tidy
```

## Pre-commit Checks

The repository has comprehensive pre-commit hooks that run:
- Formatting (gofmt, goimports)
- go mod tidy
- Linting
- Tests

These run automatically on commit.