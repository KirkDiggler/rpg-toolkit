#!/bin/bash

# Script to switch between different pre-commit hook versions

HOOKS_DIR="$(dirname "$0")"

echo "Available pre-commit hooks:"
echo "1. original - The original hook (runs all modules)"
echo "2. optimized - Only checks changed modules (default)"
echo "3. advanced - Parallel execution with caching"
echo ""
read -p "Which version? [1-3, default=2]: " choice

case ${choice:-2} in
    1)
        echo "Switching to original pre-commit hook..."
        cp "$HOOKS_DIR/pre-commit.backup" "$HOOKS_DIR/pre-commit"
        ;;
    2)
        echo "Using optimized pre-commit hook..."
        # Already in place
        ;;
    3)
        echo "Switching to advanced pre-commit hook..."
        cp "$HOOKS_DIR/pre-commit-advanced" "$HOOKS_DIR/pre-commit"
        echo ""
        echo "Advanced hook configuration:"
        echo "  PRE_COMMIT_PARALLEL=4     # Number of parallel jobs"
        echo "  PRE_COMMIT_CACHE=true     # Enable caching"
        echo "  PRE_COMMIT_VERBOSE=true   # Show debug info"
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac

echo "Done! Current pre-commit hook:"
ls -la "$HOOKS_DIR/pre-commit"