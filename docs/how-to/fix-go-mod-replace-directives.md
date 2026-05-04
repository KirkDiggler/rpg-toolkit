---
name: how to fix go.mod replace directives
description: Step-by-step guide for removing local replace directives from mechanics/* and items modules
updated: 2026-05-02
---

# How to fix go.mod replace directives

Four modules currently have local replace directives committed to main (issue #613):
- `items/go.mod` — 1 directive
- `mechanics/proficiency/go.mod` — 1 directive
- `mechanics/conditions/go.mod` — 4 directives
- `mechanics/spells/go.mod` — 6 directives

These work locally but break CI. The workspace rule is explicit: no replace directives on main.

## The fix pattern

For each affected module:

### 1. Find the current published version of each dependency

```bash
# Check what version is published
GOPROXY=direct go list -m github.com/KirkDiggler/rpg-toolkit/core@latest
GOPROXY=direct go list -m github.com/KirkDiggler/rpg-toolkit/events@latest
# etc.
```

Or check the go.mod files of modules that already pin published versions:
```bash
cat /home/kirk/personal/rpg-toolkit/tools/spatial/go.mod
# tools/spatial has clean published pins — use these as reference versions
```

### 2. Remove replace directives and update require versions

In the affected `go.mod`, remove all `replace` blocks and update `require` versions to match the latest published version for each dependency.

Example before (`mechanics/conditions/go.mod`):
```
require (
    github.com/KirkDiggler/rpg-toolkit/core v0.9.0
    ...
)
replace github.com/KirkDiggler/rpg-toolkit/core => ../../core
```

After:
```
require (
    github.com/KirkDiggler/rpg-toolkit/core v0.10.0  // or whatever is latest
    ...
)
// no replace block
```

### 3. Run go mod tidy

```bash
cd /home/kirk/personal/rpg-toolkit/<module>
go mod tidy
```

This will update `go.sum` and may adjust indirect dependency versions.

### 4. Run tests

```bash
go test -race ./...
```

Tests should pass against the published versions. If they fail because the local `core` has changes that were never published, you need to publish `core` first (see the module release workflow in the Makefile).

### 5. Create one PR per module

Per the workspace rule: one issue per PR, one PR per logical unit of work. The four affected modules each need their own cleanup PR:
- `feat/fix-613-items-go-mod`
- `feat/fix-613-conditions-go-mod`
- `feat/fix-613-proficiency-go-mod`
- `feat/fix-613-spells-go-mod`

Or combine them if the fix is trivial and the review is simple.

### 6. Verify CI passes

After pushing, ensure CI passes before merging. The key CI check is `go mod tidy` producing no diff.

## Why this matters

Local replace directives mean the module resolves its dependencies from local filesystem paths. When CI checks out only the affected module, the local paths don't exist, and the build fails. Even when CI checks out the full repo, `go mod tidy` produces a diff (the replace directives themselves) which some CI configurations treat as a failure.

The workspace CLAUDE.md is explicit: "NEVER add replace directives — Breaks CI/CD."
