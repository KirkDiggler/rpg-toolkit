---
name: tools/spawn module
description: 4-phase entity spawn engine — selection, patterns, constraints, environment integration (toolkit-internal; not directly imported by rpg-api)
updated: 2026-05-04
confidence: medium-high — verified by reading go.mod and test file names; logic verified through quality.md first-pass; consumer view per audit 049
---

# tools/spawn module

**Path:** `tools/spawn/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/tools/spawn`
**Grade:** B

> **Consumer status (per audit 049): rpg-api does NOT directly import
> `tools/spawn`.** Spawning is reached through the dnd5e dungeon flow
> (`rulebooks/dnd5e/dungeon` and friends), which then invokes the spawn
> engine internally. From the rpg-api boundary view this module is
> toolkit-internal infrastructure.

Four-phase spawn engine for placing entities in rooms during dungeon
generation. Each phase adds capability; all four are implemented and tested.

## Phases

| Phase | Implementation | Tests |
|---|---|---|
| 1: Basic engine | `basic_engine.go` | `basic_engine_test.go` |
| 2: Advanced patterns | `spawning_patterns.go` | Tested indirectly through engine |
| 3: Constraints | `constraints.go` | `constraints_test.go` |
| 4: Environment integration | `environment_integration.go` | `environment_integration_test.go` |

## Patterns (Phase 2)

`spawning_patterns.go` implements:
- **Formation** — place entities in tactical formations (line, circle, flanking)
- **Team-based** — group spawning for monster packs
- **Player choice** — deferred placement until player decides
- **Clustered** — density-based spawning with proximity constraints

These patterns have **no standalone tests** — they are exercised through the
basic engine integration tests. This is the primary quality gap.

## Constraints (Phase 3)

`constraints.go` implements spatial constraints that filter spawn candidates:
- Line-of-sight constraint
- Wall proximity constraint
- Area-of-effect avoidance
- Minimum separation from existing entities

## Environment integration (Phase 4)

`environment_integration.go` connects the spawn engine to `tools/environments`:
- `capacity_analysis.go` — analyze room capacity before spawning
- Room scaling based on actual entity count
- Split-room recommendations when capacity is exceeded

## go.mod status

Clean. Uses published versions:
- `tools/spatial v0.2.1`
- `tools/environments v0.1.2`

No replace directives. This is the cleanest dependency chain in the tools layer.

## Known gaps

- `spawning_patterns.go` and `capacity_analysis.go` have no standalone tests. A bug in formation logic would not be caught until it manifests in the encounter.
- No documented behavior for spawning in gridless rooms — the spawn engine was designed with hex/square rooms in mind.

## Verification

```sh
# rpg-api does not import tools/spawn
grep -rln '"github.com/KirkDiggler/rpg-toolkit/tools/spawn"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 0
```
