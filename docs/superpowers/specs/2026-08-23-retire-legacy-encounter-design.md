# Retire the Top-Level Legacy Encounter Module

**Status:** Approved in session on 2026-08-23  
**Issue:** [KirkDiggler/rpg-toolkit#1215](https://github.com/KirkDiggler/rpg-toolkit/issues/1215)  
**Blocks:** [KirkDiggler/rpg-toolkit#1198](https://github.com/KirkDiggler/rpg-toolkit/issues/1198)

## Purpose

Delete the independently versioned top-level
`github.com/KirkDiggler/rpg-toolkit/encounter` module. The game server already
runs on the active `rulebooks/dnd5e/session` stack, while the legacy module is
the remaining production source consumer of executable character and monster
action objects. Removing it lets #1198 replace those executable actions with
inert action data without preserving a frozen compatibility consumer.

This is a retirement, not a migration. The old module receives no adapter,
forwarder, deprecation shim, or final compatibility release.

## Verified current state

- rpg-api PRs #801 and #804 removed the old EncounterService stack and its
  dependency on the top-level toolkit module.
- `go mod why -m github.com/KirkDiggler/rpg-toolkit/encounter` in current
  rpg-api reports that the main module does not need it.
- The current rpg-api module graph contains no top-level legacy encounter
  module. It pins the active D&D 5e rulebook, encounter composition, and session
  modules instead.
- The ordered workspace has no current Go source or `go.mod` consumer outside
  the module being deleted.
- The legacy module contains 177 tracked files and approximately 50,000 lines.
- The active `rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/resolution`, and
  `rulebooks/dnd5e/session` modules are separate modules and remain supported.

## Goals

1. Remove the complete top-level `encounter/` Go module and its tests.
2. Stop current toolkit documentation from presenting that module as supported.
3. Preserve historical rationale while making supersession unmistakable.
4. Prove every remaining toolkit module still builds, tests, lints, and has
   tidy module metadata.
5. Leave #1198 free to remove the executable action model.

## Non-goals

- Implement any part of ADR-0045 or #1198.
- Change the active D&D 5e encounter composition, resolution package, or
  session host seam.
- Change rpg-api runtime behavior; its session cutover is already complete.
- Rewrite historical ADRs, journeys, plans, or issue history as though the old
  design never existed.
- Carry a deprecated package, module shim, forwarding import, or compatibility
  tag.
- Fold separately owned rpg-api, rpg-deployment, or rpg-project cleanup into
  the toolkit PR.

## Chosen approach

Perform a direct hard deletion in rpg-toolkit and update only current toolkit
truth in the same PR. Route stale cross-repository developer tooling to
separately owned follow-ups so it does not delay the source-level prerequisite
for #1198.

Rejected alternatives:

- **Clean every cross-repository reference before deleting:** complete but
  unnecessarily blocks #1198 on multiple repositories and on unavailable
  Project 19 mutations.
- **Keep a deprecated or frozen module:** preserves the dependency #1215 exists
  to remove.
- **Move the module under an archive directory:** leaves compilable source and
  creates an ambiguous second support surface instead of retiring it.

## Source changes

Delete `encounter/` in full, including:

- its `go.mod` and `go.sum`;
- aggregate, broker, transport, perception, event, and persistence source;
- executable character/monster action integration;
- tests, fixtures, commands, and the old dungeonspec workbench.

No source is moved into another module. Any capability still needed by the game
already has an independently implemented owner in the active composition,
resolution, or session stack; this slice does not reconcile implementations.

## Current documentation changes

- **`README.md`:** remove the top-level encounter module from the current module
  map. Name the active D&D 5e composition/resolution/session modules. Remove the
  drift-prone hard-coded module-root count rather than replacing it with another
  manually maintained number.
- **`docs/architecture/components/encounter.md`:** retain the path as a
  historical tombstone so existing links do not break. Put a prominent retired
  and superseded notice before the preserved historical description, and point
  current readers to the active D&D 5e modules.
- **`docs/quality.md`:** remove the retired module from active grading and the
  active grade summary. Preserve only a concise retirement note if needed for
  historical interpretation.
- **`docs/status.md`:** mark the top-level module retired in current status and
  remove it from the active subsystem inventory. Keep dated historical entries
  intact.
- **ADR-0034 and `docs/adr/DECISIONS.md`:** record that the proposed split of the
  old module was superseded by building the session stack separately and
  deleting the unconsumed module. Preserve the general lesson and the original
  decision text as history.

Other ADRs, journeys, and plans remain unchanged unless they currently claim the
legacy module is supported. Historical mention alone is not drift.

## Repository tooling and module census

Toolkit test, lint, tidy, tag, and version-census commands discover modules from
`go.mod` files dynamically. Deleting `encounter/go.mod` therefore removes the
module from those commands without a hard-coded allowlist edit. Verification
will confirm this behavior; no new census implementation is needed.

The pull request receives the `full-check` label. The changed-module workflow
cannot associate deleted Go files with a surviving `go.mod`, so without that
label it intentionally produces an empty changed-module matrix. Full checks are
required before merge.

## Cross-repository findings

Workspace references requiring separately owned disposition include:

- rpg-api's local toolkit override helper and its contract tests;
- rpg-deployment's isolated toolkit override lab, tests, and local-development
  instructions;
- rpg-project's local-game runbook and an approved historical contributor
  sandbox plan.

These references do not make rpg-api consume the module and do not pin an older
D&D 5e rulebook. They are stale contributor tooling. Record the inventory on
#1215, then route changes through separately owned issues and PRs when Project
19 is available. Current source comments and architecture tombstones that
explicitly say the old stack was removed are accurate history and are not
cleanup targets.

The external cleanup does not block the toolkit deletion or #1198.

## Delivery sequence

1. Work in an isolated `feat/1215-retire-legacy-encounter` worktree created from
   freshly fetched `origin/main`.
2. Capture baseline evidence that no current Go consumer exists.
3. Delete the legacy module and update current toolkit documentation.
4. Run complete local verification across all remaining modules.
5. If documentation PR #1214 merges first, merge updated `origin/main` into the
   feature branch; never rebase the feature branch.
6. Open one rpg-toolkit PR with `Closes #1215`, the `full-check` label, and an
   explicit note that it unblocks #1198.
7. Record external tooling findings without creating unboarded companion work
   while Project 19 is unavailable.

## Verification

### Consumer and deletion proof

- Scan Go source and every `go.mod` in the ordered workspace, excluding Git
  metadata and independent worktrees, for the exact legacy module path.
- Re-run `go mod why` and the module-graph check in rpg-api.
- Assert `encounter/` no longer exists in the feature tree.
- Assert no remaining toolkit Go source or module metadata imports or requires
  the retired path.
- Run the dynamic version census and confirm it does not list the retired
  module.

If a real consumer appears, stop. Do not delete beneath an active consumer or
silently convert it in this slice.

### Remaining repository health

Run from the toolkit repository root:

- `make pre-commit`
- `make test-all`
- `make lint-all`
- `./scripts/check-decisions.sh`
- `./scripts/check-versions.sh`
- `git diff --check origin/main...HEAD`

Also inspect `git status` after commands that can format or tidy modules. No
local `replace` directive, generated artifact, coverage file, or unrelated
module change may enter the commit.

## Acceptance criteria

- The top-level `encounter/` module is absent.
- No current ordered-workspace Go consumer requires or imports its module path.
- Active toolkit docs name only supported modules and clearly distinguish the
  retired module from `rulebooks/dnd5e/encounter`.
- Historical design material remains available with accurate supersession.
- Dynamic module census and repository checks operate only on remaining
  modules.
- All applicable local and GitHub full checks pass.
- #1198 can remove executable actions without preserving the retired module's
  dependency path.
