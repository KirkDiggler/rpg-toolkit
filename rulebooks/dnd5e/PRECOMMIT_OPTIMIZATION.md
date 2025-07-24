# Pre-commit Hook Optimization Plan

## Current Problem
The pre-commit hook runs linting and tests on ALL modules in the monorepo, even if only one file changed. This is slow and unnecessary.

## Proposed Solution

### 1. Detect Changed Modules
```bash
#!/bin/bash
# Get list of changed Go files
CHANGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')

if [ -z "$CHANGED_GO_FILES" ]; then
    echo "No Go files changed, skipping Go checks"
    exit 0
fi

# Find unique modules that have changes
CHANGED_MODULES=$(echo "$CHANGED_GO_FILES" | while read file; do
    dir=$(dirname "$file")
    # Walk up to find go.mod
    while [[ ! -f "$dir/go.mod" && "$dir" != "." ]]; do
        dir=$(dirname "$dir")
    done
    if [[ -f "$dir/go.mod" ]]; then
        echo "$dir"
    fi
done | sort -u)
```

### 2. Run Checks Only on Changed Modules
```bash
# For each changed module
for module in $CHANGED_MODULES; do
    echo "Checking module: $module"
    
    # Format check
    (cd "$module" && go fmt ./...)
    
    # Module tidy
    (cd "$module" && go mod tidy)
    
    # Linter - only on changed packages within module
    CHANGED_PACKAGES=$(echo "$CHANGED_GO_FILES" | grep "^$module" | xargs -I {} dirname {} | sort -u | sed "s|^$module|.|")
    (cd "$module" && golangci-lint run $CHANGED_PACKAGES)
    
    # Tests - only on changed packages
    (cd "$module" && go test $CHANGED_PACKAGES)
done
```

### 3. Smart Caching
```bash
# Cache linter results
export GOLANGCI_LINT_CACHE=/tmp/golangci-cache

# Use git hash for cache invalidation
MODULE_HASH=$(cd "$module" && find . -name "*.go" -type f | xargs md5sum | md5sum | cut -d' ' -f1)
CACHE_FILE="/tmp/precommit-cache/$module-$MODULE_HASH"

if [[ -f "$CACHE_FILE" ]]; then
    echo "Using cached results for $module"
    continue
fi

# Run checks and cache success
if run_checks_for_module "$module"; then
    mkdir -p $(dirname "$CACHE_FILE")
    touch "$CACHE_FILE"
fi
```

### 4. Parallel Execution
```bash
# Run module checks in parallel
export -f check_module
echo "$CHANGED_MODULES" | xargs -P 4 -I {} bash -c 'check_module "$@"' _ {}
```

### 5. Progressive Enhancement
- Quick checks first (formatting)
- Expensive checks last (tests)
- Fail fast on first error

## Full Optimized Pre-commit Hook

```bash
#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get changed Go files
CHANGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

if [ -z "$CHANGED_GO_FILES" ]; then
    echo "No Go files staged"
    exit 0
fi

# Find affected modules
get_module_for_file() {
    local file=$1
    local dir=$(dirname "$file")
    while [[ ! -f "$dir/go.mod" && "$dir" != "." ]]; do
        dir=$(dirname "$dir")
    done
    [[ -f "$dir/go.mod" ]] && echo "$dir"
}

CHANGED_MODULES=$(echo "$CHANGED_GO_FILES" | while read file; do
    get_module_for_file "$file"
done | sort -u)

echo -e "${YELLOW}Checking ${#CHANGED_MODULES[@]} modules with changes${NC}"

# Check each module
for module in $CHANGED_MODULES; do
    echo -e "\n${YELLOW}Checking $module${NC}"
    
    # Get changed packages in this module
    MODULE_FILES=$(echo "$CHANGED_GO_FILES" | grep "^$module/")
    MODULE_PACKAGES=$(echo "$MODULE_FILES" | xargs -I {} dirname {} | sort -u | sed "s|^$module/|./|")
    
    # Format (quick)
    echo "→ Formatting..."
    (cd "$module" && go fmt $MODULE_PACKAGES)
    
    # Tidy (quick) 
    echo "→ Tidying..."
    (cd "$module" && go mod tidy)
    
    # Lint (medium)
    echo "→ Linting..."
    (cd "$module" && golangci-lint run $MODULE_PACKAGES)
    
    # Test (slow - only if everything else passes)
    echo "→ Testing..."
    (cd "$module" && go test -race $MODULE_PACKAGES)
done

# Check for modifications from formatting/tidy
if [[ -n $(git diff --name-only) ]]; then
    echo -e "\n${RED}Files were modified by formatting/tidy${NC}"
    echo "Run 'git add -u' and commit again"
    exit 1
fi

echo -e "\n${GREEN}✅ All checks passed!${NC}"
```

## Benefits

1. **Speed**: Only check what changed
2. **Scalability**: Works well as monorepo grows  
3. **Developer Experience**: Fast feedback loop
4. **CI Alignment**: Same checks run in CI on the full codebase

## Implementation Notes

- Consider making this opt-in first with an env var
- Add metrics to track time saved
- Could extend to other file types (proto, yaml, etc)
- Consider integration with git worktree for truly parallel execution