---
name: rpg-toolkit architecture overview
description: Module dependency map + rules-vs-room seam (two diagrams), persistence pattern, and the boundary with rpg-api — lead-framed with toolkit-as-product
updated: 2026-07-04
confidence: high — diagrams verified against go.mod files 2026-07-04; encounter boundary per ADR-0034
---

# rpg-toolkit architecture overview

## Toolkit as product

rpg-toolkit is the product. Any game host using the toolkit should be able to ship their game by writing UI and content — the engine and its delivery shape should feel small. rpg-api (our reference game server) and rpg-dnd5e-web (our reference Discord Activity UI) are not platform components; they are the worked example for one host shape (gRPC + Redis + React + Discord). A different host — a single-player desktop client, a different multiplayer transport, a tabletop simulator — should be able to point at the same toolkit without touching it.

That framing has two operational consequences worth naming up-front, since they steer every SDK design call:

1. **When rpg-api grows complex, that is signal the toolkit is missing a primitive.** The boundary rule below ("toolkit knows rules, api orchestrates by key") is a constraint that catches drift: any rule logic that finds its way into rpg-api should produce a toolkit issue, not a rpg-api fix.
2. **Verb shape lives in the SDK, not the orchestrator.** Wave 2.11d's `PhasedCombatResolver` is the canonical example: the host implements a resolver, but the SDK owns the verb sequence (`TakeActionPhased` → `CompleteTakeAction`), the persistence shape (`PendingReactionPrompts`), and the pause-and-resume contract (`errNPCPausedForReaction`). Wave 2.11e extends the same pattern to a second resolver verb (`MovementResolver`). A new host doesn't have to re-derive any of this — they implement the resolver interface and the SDK does the rest.

## Mandate

rpg-toolkit is a Go rules engine for tabletop RPG mechanics. Its mandate is to implement game rules and return rich breakdowns. It never orchestrates data, never persists state, and never knows about rpg-api or proto definitions. If a caller (rpg-api) needs to know what Rage does, it asks the toolkit.

## Layer rules & module dependency map

**Higher layers may import lower; reversing is a defect.** The diagram shows the
**decided end-state** (ADR-0034 — the *split*): the D&D-5e encounter loop lives
**inside** the `rulebooks/dnd5e` module, and the encounter's already-agnostic
layers become their own primitive modules **below** the rulebook. Everything sits
at or below the Rulebooks layer, and **nothing imports rpg-api or rpg-api-protos**
(the toolkit never knows its host). Dashed nodes flag the migration that is
decided but not yet executed.

```mermaid
flowchart TD
  subgraph RB["rulebooks/dnd5e module — D&D 5e rules + encounter loop"]
    DND["dnd5e rules<br/>character · combat · monsters · conditions"]
    ENC["encounter loop<br/>verbs · hydration · resolvers · turn logic"]
    ENC --> DND
  end
  subgraph PRIM["New agnostic primitives (extracted from the encounter)"]
    BROKER["broker<br/>pub/sub + timestamp + Transport seam"]
    SPINE["eventspine<br/>sealed event interface + audience routing"]
    PERC["tools/perception<br/>vision projection"]
  end
  subgraph TOOLS["Tools"]
    SPATIAL["tools/spatial<br/>hex (absorbs encounter/core)"]
    ENV["tools/environments"]
    SEL["tools/selectables"]
    SPAWN["tools/spawn"]
  end
  subgraph MECH["Mechanics"]
    EFFECTS["mechanics/effects"]
    COND["mechanics/conditions"]
    RES["mechanics/resources"]
    FEAT["mechanics/features"]
    PROF["mechanics/proficiency"]
    SPELLS["mechanics/spells"]
  end
  subgraph CORELYR["Core"]
    C["core"]
    EV["events"]
    DICE["dice"]
    GAME["game"]
    ERR["rpgerr"]
    ITEMS["items"]
  end

  ENC --> BROKER & SPINE & PERC
  DND --> RES & ENV & SPATIAL & EV & C & DICE
  BROKER --> SPINE & EV & C
  SPINE --> EV & C
  PERC --> SPATIAL
  SPAWN --> ENV & SPATIAL
  ENV --> SPATIAL
  SPATIAL --> GAME & EV & C
  COND --> EFFECTS
  EFFECTS --> EV & C

  MIG["MIGRATION PENDING — ADR-0034<br/>Today: one top-level 'encounter' module requiring rulebooks/dnd5e<br/>by pseudo-version. Post-Beat-2 its dnd5e files merge INTO the<br/>rulebooks/dnd5e module and its agnostic layers become the<br/>primitives shown (broker · eventspine · tools/perception)."]
  ENC -.->|"scheduled split"| MIG

  style ENC stroke:#d33,stroke-width:2px,stroke-dasharray:6 4
  style BROKER stroke:#2a7,stroke-width:2px,stroke-dasharray:6 4
  style SPINE stroke:#2a7,stroke-width:2px,stroke-dasharray:6 4
  style PERC stroke:#2a7,stroke-width:2px,stroke-dasharray:6 4
  style MIG fill:#fff3cd,stroke:#d39e00,color:#663c00
```

Nodes are modules — **except** `encounter loop`, which in the end-state is a
*package inside* the `rulebooks/dnd5e` module (drawn separately to show the seam).
Green dashed nodes are the new agnostic primitive modules the split creates; the
red dashed loop is the dnd5e code merging into the rulebook module. Arrows are
dependencies; uniform ones are omitted for legibility — the loop shares the
`rulebooks/dnd5e` module's dependencies (`core`, `dice`, `events`, `tools/spatial`,
…), every Mechanics module requires `core` + `events` (`conditions` also
`effects`, shown), and `rpgerr` / `game` / `items` are Core-layer leaves. **Maintenance
rule:** update this diagram in the *same* PR as any module move — including when
the pending split lands. The prose list this replaced went stale (it claimed
"nothing depends on `rulebooks/dnd5e`" long after the encounter did); a diagram
that travels with the change cannot.

## The persistence pattern

Every stateful toolkit component implements exactly two methods:

```go
func (c *Character) ToData() *Data { ... }
func LoadFromData(ctx context.Context, data *Data, bus events.EventBus) (*Character, error) { ... }
```

The data orchestrator (rpg-api) holds the serialized `Data` struct (typically as JSON in Redis). When it needs the character to participate in a rule call, it calls `LoadFromData` to reconstitute the live object, invokes the rule, then calls `ToData` again to serialize the result. **The toolkit never opens a database connection, never reads from Redis, and never persists anything.**

This pattern is present in:
- `rulebooks/dnd5e/character/character.go` — `Character.ToData()` / `LoadFromData()`
- `rulebooks/dnd5e/character/draft_data.go` — `Draft.ToData()` / `LoadDraftFromData()`
- `tools/environments/environment_persistence.go` — `BasicEnvironment.ToData()` / `LoadFromData()`
- `tools/spatial/data.go` — `RoomData` serialization, `LoadRoomFromContext()`

Conditions and features use a JSON variant of the same pattern:
- `ToJSON() (json.RawMessage, error)` — serialize active state
- `LoadJSON(data json.RawMessage) (ConditionBehavior, error)` — reconstitute from JSON blob

The difference: `LoadFromData` is used when the data schema is homogeneous (one struct type per entity); `LoadJSON` is used when the loader routes by ref (multiple condition/feature types serialized to the same opaque blob field).

## The boundary rule

```
Client sends REFERENCES (keys, IDs) → never calculations
API orchestrates by KEY             → never knows what Rage does
Toolkit implements RULES            → returns rich breakdowns for rendering
```

Toolkit's job ends when it returns a `Breakdown` struct. The breakdown contains the full modifier chain — base value, per-modifier deltas, sources, labels — so the UI can render the reasoning without re-implementing rules.

## The dnd5e encounter: rules, loop, and primitives

Post-split (ADR-0034), the seam runs three ways. Inside the `rulebooks/dnd5e`
module the **rules** and the **loop** coexist as packages: the loop asks the rules
for verdicts and publishes the turn-tick vocabulary; the rules return receipts and
per-entity state. The loop then *composes* the agnostic **primitives** the split
extracts below it.

```mermaid
flowchart LR
  subgraph MOD["rulebooks/dnd5e module (post-split)"]
    direction TB
    subgraph RULES["what the RULES say"]
      R1["Resolution math · saves"]
      R2["Condition behaviors"]
      R3["Per-entity state<br/>economy · HP · once-per-turn"]
      R4["Breakdown receipts"]
    end
    subgraph LOOP["the dnd5e LOOP — orchestration"]
      E1["Turn loop + tick publishes"]
      E2["Hydration cascade"]
      E3["Verb dispatch — TakeAction"]
      E4["Audience / observer-set decisions"]
    end
    LOOP -->|"asks verdicts · publishes ticks<br/>TurnStart · TurnEnd (live) · combat-ended (designed #596)"| RULES
    RULES -->|"Breakdown + per-entity state"| LOOP
  end
  subgraph PRIM["agnostic PRIMITIVES — extracted below"]
    direction TB
    P1["broker + transport<br/>delivery"]
    P2["eventspine<br/>sealed events + audience routing"]
    P3["tools/perception · tools/spatial<br/>vision + hex"]
  end
  LOOP ==>|"composes"| PRIM

  linkStyle 2 stroke:#2a7,stroke-width:2px,color:#2a7
```

The rules-vs-loop seam is a **package boundary within one module** — no
cross-module pseudo-version tax. The primitives (`broker`, `eventspine`,
`tools/perception`, and hex folded into `tools/spatial`) are reusable modules
*beneath* the rulebook, following the pattern already set by `tools/spatial` (hex)
and `mechanics/effects` (of which dnd5e conditions are the rulebook-specific
expression). The split is honest on both sides: dnd5e code with the rules, generic
infrastructure at the general layer.

## Module map

| Module | Path | Layer | Purpose |
|---|---|---|---|
| core | `core/` | Core | Entity, EntityType, Ref, TypedRef, Action, chain types |
| rpgerr | `rpgerr/` | Core | Structured error accumulation with RPG context |
| dice | `dice/` | Core | Roller, Pool, LazyRoll, Modifier, Notation |
| events | `events/` | Core | EventBus, BusEffect, TypedTopic, ChainedTopic |
| game | `game/` | Core | game.Context pattern for passing game state through event chains |
| items | `items/` | Core | Item/EquippableItem/WeaponItem interfaces — base only, no implementation |
| mechanics/effects | `mechanics/effects/` | Mechanics | Shared effect infrastructure (tracker, behaviors) |
| mechanics/conditions | `mechanics/conditions/` | Mechanics | Condition manager, simple/enhanced condition types |
| mechanics/resources | `mechanics/resources/` | Mechanics | Resource pools (spell slots, ki, rage uses) |
| mechanics/features | `mechanics/features/` | Mechanics | Feature loader infrastructure |
| mechanics/proficiency | `mechanics/proficiency/` | Mechanics | Proficiency system |
| mechanics/spells | `mechanics/spells/` | Mechanics | Spell slots, concentration, spell list |
| tools/spatial | `tools/spatial/` | Tools | Hex/Square/Gridless room + multi-room orchestration |
| tools/environments | `tools/environments/` | Tools | Environment persistence, graph generation, multi-room dungeon graph |
| tools/selectables | `tools/selectables/` | Tools | Weighted random selection tables |
| tools/spawn | `tools/spawn/` | Tools | 4-phase entity spawn engine |
| rulebooks/dnd5e | `rulebooks/dnd5e/` | Rulebooks | Full D&D 5e rules: character, combat, initiative, spells, monsters, dungeon — **plus the encounter loop** (post-split) |
| encounter loop | `rulebooks/dnd5e/encounter/` *(migration pending; today `encounter/`)* | Rulebooks (D&D 5e) | The dnd5e game loop: turn loop, hydration cascade, resolver seam, verb dispatch, prompts. A **package inside** the dnd5e module in the end-state — merges in per ADR-0034 (kills the pseudo-version dance). |
| broker | `broker/` *(new; migration pending)* | Primitive | Pub/sub fan-out + game-event timestamp authority + the `Transport` seam. Extracted from the encounter (ADR-0034). |
| eventspine | `eventspine/` *(new; migration pending)* | Primitive | Sealed event-interface mechanism + audience routing. Extracted from the encounter; concrete dnd5e events stay with the loop (ADR-0034). |
| tools/perception | `tools/perception/` *(new; migration pending)* | Tools | Vision projection over `tools/spatial`. Extracted from the encounter (ADR-0034). |

## Code violations against these rules (2026-05-02)

### Rule: No local `replace` directives on main
**Violated by four modules committed to main:**
- `items/go.mod` — `replace github.com/KirkDiggler/rpg-toolkit/core => ../core`
- `mechanics/conditions/go.mod` — 4 replace directives (`core`, `dice`, `events`, `effects`)
- `mechanics/proficiency/go.mod` — `replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects`
- `mechanics/spells/go.mod` — 6 replace directives (`core`, `dice`, `events`, `conditions`, `effects`, `resources`)

Tracked in issue #613. These work locally but break CI because published module resolution fails when directives are present.

### Rule: Higher layers only; Tools is not Rulebooks
**Potential violation: `rulebooks/dnd5e/dungeon/` inside the rulebook:**
The `dungeon/` package (`dungeon.go`, `dungeon_data.go`, `types.go`) provides procedural dungeon generation that architecturally belongs at the Tools layer (so rpg-api can use dungeon logic without importing the full dnd5e rulebook). It uses `tools/environments` and `tools/spatial` (both lower layers — that direction is correct), but its location makes `rulebooks/dnd5e` the only consumer path. The planned move is to `tools/dungeon/` or a new top-level module. No issue or branch exists yet.

### Rule: Test coverage at the rule layer
**Violated by `rulebooks/dnd5e/backgrounds/grants.go` (172 lines) and `rulebooks/dnd5e/races/grants.go` (109 lines):**
Both files implement grant logic (skill proficiencies, language grants, weapon and armor grants by race). Neither has a `*_test.go` file. Verified: `find rulebooks/dnd5e/backgrounds -name "*_test.go"` returns empty. Tracked in issue #615.

**Violated by `mechanics/features/` base module:**
`loader.go`, `feature.go`, `simple_feature.go` — no test files in the base module. Only `mock/` exists. Feature loader is tested indirectly via `rulebooks/dnd5e/features`. Direct unit tests for routing and error paths are absent.

### Rule: Square-grid intra-room pathfinding
**Gap: `PathFinder` interface is hex-only (`tools/spatial/pathfinder.go:9`):**
```go
FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
```
`CubeCoordinate` is the hex coordinate type. There is no `SquarePathFinder`. A monster navigating obstacles inside a square room has no toolkit path to follow. The orchestrator's `FindPath` is room-to-room only. Tracked in issue #614.

## Testing approach

- **Testify suite pattern** — `suite.Suite` + `SetupTest()` + `s.Run()` throughout
- **Uber gomock** — mocks in `mock/` subdirectory per module
- **Per-module tests** — each module is tested in isolation: `cd <module> && go test ./...`
- **Integration tests** — `rulebooks/dnd5e/integration/` covers full Barbarian/Fighter/Monk/Rogue encounter scenarios

Run tests:
```bash
# Single module
cd /home/kirk/personal/rpg-toolkit/core && go test -race ./...

# All modules (Makefile target)
make test-all

# Full pre-commit (fmt + tidy + lint + test for core + events)
make pre-commit
```

Note: `make pre-commit` only covers `core` and `events`. For other modules run per-module `go test ./...` and `golangci-lint run ./...` manually before committing.
