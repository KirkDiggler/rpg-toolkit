---
name: mechanics modules
description: Effects and resources — mechanics infrastructure; rulebook features and proficiencies have their own implementations
updated: 2026-05-04
confidence: medium-high — verified by reading go.mod files, key source files, and the rpg-api import graph per audit 049
---

# mechanics modules

> **Consumer status (per audit 049): rpg-api does NOT directly import any
> `mechanics/*` module.** These modules are toolkit-internal architecture —
> depended on by `rulebooks/dnd5e/*`, which is the consumer-facing layer. From
> the rpg-api boundary view, mechanics is implementation detail.
>
> Several mechanics modules carry `replace` directives in their `go.mod`
> pointing `events => ../../events`. Their source still references old-API
> symbols (`events.Event`, `events.HandlerFunc`) that neither the local nor
> any published events module exposes today, so these modules don't build
> against either pinned or local events. Closing **issue #617** means
> rewriting them against the typed-topic API (`TypedTopic[T]`,
> `ChainedTopic[T]`, `BusEffect`, `StagedChain`). The 4-class playtest
> doesn't exercise these modules directly, so the migration is deferred.
>
> See `events.md` for the new events API and `docs/journey/049-rpg-api-toolkit-usage-audit.md`
> for the consumer-side usage data.

Current mechanics modules are `effects` and `resources`; see
[the checked module map](../../../knowledge/map.md). The conditions/spells
sections below describe already-retired modules, not supported current APIs.

## mechanics/effects — B

**Path:** `mechanics/effects/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/effects`

Shared infrastructure for conditions and proficiencies. Provides
`EffectTracker`, effect behaviors, and a composed condition base type.

### Key types
- `EffectTracker` — tracks applied/removed effects with deduplication
- `ComposedEffect` — base struct for effects that compose multiple behaviors
- `BehaviorFn` — function type for effect behavior handlers

### Known gaps
- Tests are flat (`TestXxx`) not suite pattern — inconsistent with repo standard
- No doc distinguishing when to use `mechanics/effects` vs. `mechanics/conditions` directly. The answer: `effects` is the shared infrastructure; `conditions` consumes it.
- No go.mod issues. Clean dependency tree.

---

## mechanics/conditions — B

**Path:** `mechanics/conditions/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/conditions`

Condition manager plus simple/enhanced condition types.

### Key types
- `Manager` — tracks active conditions per entity; handles apply/remove/query
- `SimpleCondition` — basic BusEffect with apply/remove handlers
- `EnhancedCondition` — SimpleCondition with stacking and duration support

### go.mod state (issue #617)
`mechanics/conditions/go.mod` carries four committed `replace` directives,
including `events => ../../events`. The local `events` module on this branch
exposes only the typed-topic API (`TypedTopic[T]`, `ChainedTopic[T]`,
`BusEffect`, `StagedChain`) — not the old `events.Event` / `events.HandlerFunc`
/ `event.Context().GetString` / `.AddModifier()` symbols this module's source
still references. So the replace directives don't actually let this module
build today; running `go mod tidy` or `go build ./...` inside
`mechanics/conditions` fails. Resolution is deferred to **issue #617**, which
means rewriting the source against the typed-topic API — a real refactor, not
a version bump. The 4-class playtest doesn't exercise this module (it
consumes `rulebooks/dnd5e/conditions` instead, which already runs against the
new API), so this is on hold until the base module is needed.

### Coverage note
Good behavior coverage at the `rulebooks/dnd5e` level (raging, dodging,
unconscious, etc. all exercised in integration tests), but the base
`Manager`/`SimpleCondition`/`EnhancedCondition` tests are flat and not
suite-pattern.

---

## mechanics/resources — B+

**Path:** `mechanics/resources/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/resources`

Resource pools and counters for finite game resources.

### Key types
- `Pool` — finite resource with max, current, refill-on-rest semantics (spell slots, ki points)
- `Counter` — simpler counter (rage uses, second wind)

### Status
No go.mod issues. Tests cover happy paths. Edge cases (refill to zero,
consume past limit) not explicitly tested, but the pool logic is
straightforward enough that this is low risk.

---

## mechanics/spells — B-

**Path:** `mechanics/spells/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/spells`

Spell slots, concentration tracking, spell lists.

### Key types
- `SlotManager` — tracks spell slot usage by level
- `ConcentrationTracker` — manages concentration (one spell at a time)
- `SpellList` — list of known/prepared spells

### go.mod state (issue #617)
Six replace directives committed to main. Same situation as
`mechanics/conditions`: source uses old-API events symbols that don't exist
in any published events version. Closing #617 means rewriting against the
new typed-topic API. Tracked in **issue #617**, deferred until the playtest
exercises spells.

### Coverage note
Concentration logic, spell events, and slot management all have test files
and pass. Test style is mostly flat (not suite pattern). No known logic bugs.
