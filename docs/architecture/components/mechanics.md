---
name: mechanics modules
description: Conditions, effects, features, proficiency, resources, spells — the modifier pipeline infrastructure
updated: 2026-05-02
confidence: medium-high — verified by reading go.mod files, key source files, and test runs per module
---

# mechanics modules

Six sub-modules that implement the D&D/RPG modifier pipeline infrastructure. All depend on `core` and `events`; none depend on `rulebooks/dnd5e`.

## mechanics/effects — B

**Path:** `mechanics/effects/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/effects`

Shared infrastructure for conditions and proficiencies. Provides `EffectTracker`, effect behaviors, and a composed condition base type.

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

### go.mod violation (issue #613)
`mechanics/conditions/go.mod` carries four committed `replace` directives:
```
replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects
replace github.com/KirkDiggler/rpg-toolkit/events           => ../../events
replace github.com/KirkDiggler/rpg-toolkit/core             => ../../core
replace github.com/KirkDiggler/rpg-toolkit/dice             => ../../dice
```

Running `go test ./...` from this module emits `go: updates to go.mod needed` before printing test results. Tests pass locally, but CI fails on the go.mod diff. This violates the workspace rule: no replace directives on main.

### Coverage note
Good behavior coverage at the `rulebooks/dnd5e` level (raging, dodging, unconscious, etc. all exercised in integration tests), but the base `Manager`/`SimpleCondition`/`EnhancedCondition` tests are flat and not suite-pattern.

---

## mechanics/resources — B+

**Path:** `mechanics/resources/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/resources`

Resource pools and counters for finite game resources.

### Key types
- `Pool` — finite resource with max, current, refill-on-rest semantics (spell slots, ki points)
- `Counter` — simpler counter (rage uses, second wind)

### Status
No go.mod issues. Tests cover happy paths. Edge cases (refill to zero, consume past limit) not explicitly tested, but the pool logic is straightforward enough that this is low risk.

---

## mechanics/features — C

**Path:** `mechanics/features/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/features`

Feature loader infrastructure. The base module provides the routing layer; rulebooks provide implementations.

### Key types
- `FeatureData` interface — gives loaders access to a feature's `Ref()` and `JSON()`
- `Loader` — routes `FeatureData` to the correct implementation by ref
- `SimpleFeature` — a basic feature that fires a callback on activation

### go.mod: no issues
Clean dependencies — only `core`.

### Critical gap (issue #615 adjacent)
`loader.go`, `feature.go`, `simple_feature.go` — **zero test files in the base module.** Only `mock/` exists. The feature loader routing logic and error paths are untested at the module level. Indirect coverage exists through `rulebooks/dnd5e/features`, but a bug in `Loader.Route()` error handling would not be caught by running `cd mechanics/features && go test ./...` — that command prints "no test files."

Grade would move to B with tests that exercise the loader routing and error paths.

---

## mechanics/proficiency — B-

**Path:** `mechanics/proficiency/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency`

Proficiency system. Tracks what an entity is proficient with and calculates proficiency-bonus-adjusted modifiers.

### Key types
- `Proficiency` interface
- `SimpleProfiler` — concrete implementation

### go.mod violation (issue #613)
```
replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects
```

One replace directive committed to main. Tests pass locally. CI will flag the go.mod diff.

---

## mechanics/spells — B-

**Path:** `mechanics/spells/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/mechanics/spells`

Spell slots, concentration tracking, spell lists.

### Key types
- `SlotManager` — tracks spell slot usage by level
- `ConcentrationTracker` — manages concentration (one spell at a time)
- `SpellList` — list of known/prepared spells

### go.mod violation (issue #613) — most severe
Six replace directives committed to main:
```
replace github.com/KirkDiggler/rpg-toolkit/core               => ../../core
replace github.com/KirkDiggler/rpg-toolkit/dice               => ../../dice
replace github.com/KirkDiggler/rpg-toolkit/events             => ../../events
replace github.com/KirkDiggler/rpg-toolkit/mechanics/conditions => ../conditions
replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects    => ../effects
replace github.com/KirkDiggler/rpg-toolkit/mechanics/resources  => ../resources
```

This module has the most replace directives of any module in the repo. Tests pass locally; CI fails on go.mod diff. Needs a dedicated cleanup PR.

### Coverage note
Concentration logic, spell events, and slot management all have test files and pass. Test style is mostly flat (not suite pattern). No known logic bugs.
