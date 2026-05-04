---
name: how to run tests
description: Per-module test commands, known failures, pre-commit targets
updated: 2026-05-02
---

# How to run tests

rpg-toolkit is multi-module. Each module must be tested independently — there is no workspace-level `go test ./...` that covers all modules at once.

## Per-module test command

```bash
# From the module directory
cd /home/kirk/personal/rpg-toolkit/<module>
go test -race ./...
```

### All modules with known-passing tests (as of 2026-05-02)

```bash
cd /home/kirk/personal/rpg-toolkit/core && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/events && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/dice && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/rpgerr && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/game && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/mechanics/effects && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/mechanics/resources && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/mechanics/features && go test -race ./...  # "no test files" -- not a failure
cd /home/kirk/personal/rpg-toolkit/mechanics/proficiency && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/mechanics/spells && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/tools/spatial && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/tools/environments && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/tools/selectables && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/tools/spawn && go test -race ./...
cd /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e && go test -race ./...
```

### Modules with known issues

```bash
# mechanics/conditions — runs but emits go.mod warning
cd /home/kirk/personal/rpg-toolkit/mechanics/conditions && go test ./...
# Emits: go: updates to go.mod needed (before printing test results)
# Tests pass. CI will fail on the go.mod diff.
# Tracked: issue #613
```

## Makefile targets

```bash
# pre-commit: fmt + tidy + lint + test for core + events only
make pre-commit

# test all modules (uses find to locate all go.mod files)
make test-all

# lint all modules
make lint-all

# format all modules
make fmt-all
```

Note: `make pre-commit` only covers `core` and `events`. For modules outside those two, run per-module commands before committing.

## Integration tests (dnd5e)

```bash
cd /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e
go test -race ./integration/...
```

This runs the full Barbarian/Fighter/Monk/Rogue encounter scenarios. These are the most valuable tests in the codebase — they exercise the complete chain from character finalization through combat resolution.

## Linting

```bash
# Per module
cd /home/kirk/personal/rpg-toolkit/<module>
golangci-lint run ./...

# All modules (Makefile)
make lint-all
```

## go mod tidy check

When adding or changing dependencies in any module:
```bash
cd /home/kirk/personal/rpg-toolkit/<module>
go mod tidy
```

If the command changes `go.mod` or `go.sum`, commit those changes. CI runs `go mod tidy` and fails if the diff is non-empty.

## Known CI behavior

- `make pre-commit` passes today for core + events.
- `mechanics/conditions`, `mechanics/spells` tests pass locally but CI fails because `go mod tidy` would change their go.mod (replace directives present, issue #613).
- `items` module tests now compile and pass (resolved per issue #612).
