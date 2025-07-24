# CI Optimization Guide

## Overview

We've implemented an optimized CI workflow that significantly speeds up PR checks by only testing changed modules, while maintaining full coverage for main branch merges.

## Workflows

### 1. `ci-optimized.yml` (Recommended)
- **PRs**: Only tests modules with changes
- **Main branch**: Runs full test suite
- **Special label**: Add `full-check` label to PR to force full suite

### 2. `ci.yml` (Legacy Full Checks)
- Always runs all tests on all modules
- Kept for reference/fallback

## How It Works

### Change Detection
1. Detects which Go files changed
2. Maps files to their containing modules (by finding go.mod)
3. Creates a matrix job for each changed module

### PR Workflow
```
PR with changes to rulebooks/dnd5e/:
- Only tests rulebooks/dnd5e
- Skips core, events, tools, etc.
- Result: 30 seconds instead of 5+ minutes
```

### Main Branch Workflow
```
Push/merge to main:
- Tests ALL modules
- Ensures no integration issues
- Full compatibility check
```

## Developer Benefits

1. **Faster feedback**: PRs test in seconds, not minutes
2. **Parallel execution**: Changed modules test concurrently
3. **Smart caching**: Go module cache per module
4. **Override option**: Add `full-check` label when needed

## Migration Steps

1. Rename current workflow: `ci.yml` → `ci-full.yml`
2. Rename optimized: `ci-optimized.yml` → `ci.yml`
3. Update branch protection to use `ci-status` job

## Local Development

Use the advanced pre-commit hook for similar benefits locally:
```bash
cd /home/kirk/personal/rpg-toolkit
.githooks/switch-precommit.sh
# Select option 3 (advanced)
```

This provides:
- Only check changed modules
- Parallel execution
- Caching support
- Same behavior as CI