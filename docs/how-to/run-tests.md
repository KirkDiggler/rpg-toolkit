---
name: how to run tests
description: Per-module, full-repository, lint, and pre-commit commands for the multi-module repository
updated: 2026-08-10
---

# How to run tests

RPG Toolkit has 22 independent Go module roots and no root `go.mod`. A root-level
`go test ./...` is therefore not a repository test. Run the owning module first,
then use the Makefile's module discovery when a full sweep is required.

## Owning module

From the repository root:

```bash
cd rulebooks/dnd5e       # replace with the module you changed
go test -race ./...
golangci-lint run ./...
```

For a D&D 5e monster contribution, use the more focused commands and round-trip
acceptance in [Add a D&D 5e monster](add-a-dnd5e-monster.md) before the full
rulebook command.

Other examples:

```bash
cd core && go test -race ./...
cd events && go test -race ./...
cd encounter && go test -race ./...
cd tools/spatial && go test -race ./...
cd play/clock && go test -race ./...
```

Each `cd` above starts from the repository root; run them as separate commands or
return to the root between them.

## All modules

The Makefile discovers every tracked `go.mod`:

```bash
make test-all       # go test -race ./... in every module
make lint-all       # golangci-lint in every module
make fmt-all        # writes formatting changes across all Go source
make mod-tidy       # writes dependency changes in every module
```

Use the writing targets (`fmt-all`, `mod-tidy`) deliberately and inspect the
diff. A scoped change should not commit unrelated module churn.

## D&D 5e integration tests

From `rulebooks/dnd5e`:

```bash
go test -race ./integration/...
```

These tests exercise complete rulebook flows for the currently covered classes.
They do not replace package-level tests for a new rule or content loader.

## Pre-commit gates

Run the required repository target from the root:

```bash
make pre-commit
```

The target currently formats, tidies, lints, and tests **Core and Events only**;
it does not validate all modules. Always run owning-module tests/lint too.

There is a known main-branch defect in its Core coverage extraction
([#769](https://github.com/KirkDiggler/rpg-toolkit/issues/769)): the target can
leak `go test` package output into the numeric `bc` comparison and fail even when
tests/lint pass. Record that exact failure and the successful owning-module
evidence; do not bypass hooks or claim the target passed.

The installed `.githooks/pre-commit` script is different from the Make target.
It inspects staged `.go` files, discovers their owning modules, and checks only
those changed packages. With no staged Go files (for example, a documentation-
only commit), it exits successfully after reporting that fact.

## Formatting, dependency, and diff checks

For a changed Go module:

```bash
cd <module>
gofmt -w <changed-go-files>
go mod tidy
git diff --exit-code -- go.mod go.sum   # omit --exit-code if dependency changes are intentional
```

From the repository root:

```bash
git diff --check
```

Never commit a local `replace` directive. See
[Fix `go.mod` replace directives](fix-go-mod-replace-directives.md).

## Documentation checks

There is currently no dedicated Markdown or link-check Make target. For a docs
change:

- run `git diff --check`;
- resolve every repository-relative link from the Markdown file containing it;
- check any new external URLs;
- report the method used rather than naming a nonexistent repository check.
