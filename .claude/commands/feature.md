---
description: Guide me through implementing a new feature following rpg-toolkit best practices
---

# Feature Development Workflow

I will guide you through implementing a new feature following rpg-toolkit best practices.

## Step 1: Understand Requirements

First, let me understand:
1. What feature are we implementing? (Issue number or description)
2. What modules/packages does this affect?
3. Are there any design decisions to make?

If there's an issue number, I'll fetch details from GitHub.

## Step 2: Create Feature Branch

I will:
1. Ensure we're on main branch
2. Pull latest changes from origin/main
3. Create a new branch named `feature/description` or `feat/issue-XXX`
4. Confirm branch creation

## Step 3: Design Approach (if needed)

For complex features, I'll:
1. Propose the implementation approach
2. Identify what tests we need
3. Ask clarifying questions about edge cases
4. Check for existing patterns in the codebase

## Step 4: Test-Driven Development

Following TDD approach:
1. Write tests for the new functionality FIRST
2. Verify tests fail (red)
3. Implement minimal code to make tests pass (green)
4. Refactor if needed (refactor)
5. Repeat for each component

## Step 5: Implementation

I will:
1. Follow the Input/Output pattern (from CLAUDE.md)
2. Use proper separation of concerns
3. Add validation at appropriate layers
4. Follow existing code patterns in the module

## Step 6: Documentation

I will update:
1. Code comments for public APIs
2. Package README if behavior changes
3. Journey docs if this involves exploration/decisions
4. ADR if this involves architectural decisions

## Step 7: Pre-Commit Checks

Before committing, I'll run:
```bash
make pre-commit
```

And fix any issues.

## Step 8: Commit and PR

I will:
1. Create meaningful commit message(s)
2. Push branch to origin
3. Create PR with:
   - Summary of feature
   - Test plan
   - Any breaking changes
   - Claude attribution

---

**Ready to start? Tell me what feature we're implementing!**
