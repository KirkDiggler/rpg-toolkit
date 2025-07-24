# Git Hooks

This directory contains Git hooks for the rpg-toolkit project.

## Pre-commit Hook

The pre-commit hook runs checks on staged Go files before allowing a commit.

### Available Versions

1. **Original** (`pre-commit.backup`)
   - Runs all checks on ALL modules in the repo
   - Slow but thorough
   - Good for release commits

2. **Optimized** (`pre-commit`)
   - Only checks modules with staged changes
   - Runs only on changed packages within those modules
   - Much faster for day-to-day development
   - Currently active by default

3. **Advanced** (`pre-commit-advanced`)
   - All features of optimized version
   - Parallel execution for multiple modules
   - Caching of successful checks
   - Configurable via environment variables

### Switching Versions

Run the switch script:
```bash
./.githooks/switch-precommit.sh
```

### Advanced Hook Configuration

The advanced hook supports these environment variables:

```bash
# Run with 8 parallel jobs
PRE_COMMIT_PARALLEL=8 git commit

# Disable caching
PRE_COMMIT_CACHE=false git commit

# Enable verbose output
PRE_COMMIT_VERBOSE=true git commit

# Combine options
PRE_COMMIT_PARALLEL=2 PRE_COMMIT_VERBOSE=true git commit
```

### What Gets Checked

For each module with changes:
1. **Formatting** - `go fmt` on changed packages
2. **Dependencies** - `go mod tidy` 
3. **Linting** - `golangci-lint` on changed packages
4. **Tests** - `go test -race` on changed packages

### Performance Comparison

Example with changes in 3 modules:

| Version | Time | Packages Checked |
|---------|------|------------------|
| Original | ~2m 30s | ALL packages in ALL modules |
| Optimized | ~45s | Only changed packages in 3 modules |
| Advanced (parallel) | ~20s | Same as optimized but parallel |
| Advanced (cached) | ~5s | Only newly changed packages |

### Bypassing Checks

If you need to commit without checks (not recommended):
```bash
git commit --no-verify
```

### Troubleshooting

**Hook not running?**
```bash
# Ensure hooks are enabled
git config core.hooksPath .githooks
```

**Linter not found?**
```bash
# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

**Cache issues?**
```bash
# Clear the cache
rm -rf /tmp/rpg-toolkit-precommit-cache
```

### Development

To test changes to the pre-commit hook:
```bash
# Create a test file
echo "package main" > test.go
git add test.go

# Run the hook directly
./.githooks/pre-commit

# Clean up
git reset test.go
rm test.go
```