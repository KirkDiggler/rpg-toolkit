---
description: Guide me through the complete bug fix workflow for rpg-toolkit
---

# Bug Fix Workflow

I will guide you through fixing a bug following rpg-toolkit best practices.

## Step 1: Verify Issue and Context

First, let me check:
1. What issue number are we fixing?
2. What is the current branch?
3. Are we up to date with main?

Please provide the issue number (e.g., "#346" or just "346"), and I'll:
- Fetch the issue details from GitHub
- Check current git status
- Verify we're in the correct repository location

## Step 2: Create Bug Fix Branch

Once I understand the issue, I will:
1. Ensure we're on main branch
2. Pull latest changes from origin/main
3. Create a new branch named `fix/issue-XXX` (following the pattern from CLAUDE.md)
4. Confirm branch creation

## Step 3: Write Failing Test

Before fixing anything, I will:
1. Create a test that demonstrates the bug
2. Run the test to confirm it fails
3. Show you the failing test output
4. Explain what the test is checking

**This is critical** - we need proof the bug exists and a way to verify the fix works.

## Step 4: Implement Fix

I will:
1. Analyze the root cause based on the failing test
2. Implement the minimal fix needed (following "optimize for simplicity")
3. Run the test again to verify it passes
4. Run the full test suite to ensure no regressions

## Step 5: Pre-Commit Checks

Before committing, I will run:
```bash
make pre-commit
```

This runs:
- `gofmt`
- `goimports`
- `go mod tidy`
- Linters
- Tests

If any checks fail, I'll fix them before proceeding.

## Step 6: Commit Changes

I will create a commit with:
- Clear description of the bug and fix
- Reference to the issue number
- Standard commit message format with Claude attribution

## Step 7: Create Pull Request

Finally, I will:
1. Push the branch to origin
2. Create a PR using `gh pr create`
3. Include:
   - Summary of the bug
   - Explanation of the fix
   - Link to the issue
   - Test coverage details

---

**Ready to start? Tell me the issue number!**

(If you already know the issue details, you can also paste the GitHub URL or describe the bug, and I'll skip fetching from GitHub)
