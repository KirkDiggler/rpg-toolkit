---
name: core module
description: Fundamental interfaces and types every other module depends on — entity, ref, action, errors, plus core/combat and core/resources sub-packages
updated: 2026-05-04
confidence: high — verified by reading core/ source and rpg-api import graph (audit 049)
---

# core module

**Path:** `core/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/core`
**Grade:** A-

The foundation. Every other toolkit module imports `core`. It defines the minimum
shared vocabulary for game objects, identifiers, errors, and actions.

## What rpg-api consumes

Per the audit at `docs/journey/049-rpg-api-toolkit-usage-audit.md`, rpg-api
imports the top-level `core` package from 8 files. The hot path is:

| Symbol | Where rpg-api uses it | Histogram |
|---|---|---|
| `core.Ref` | converters, entity types, encounter events | 46 occurrences |
| `core.Entity` | perception, registry plumbing | 5 |
| `core.EntityType` | perception filters, mocks | 2 |

`core.Action` does **not** appear directly in rpg-api. It's a toolkit-internal
contract — see "Action vs ConditionBehavior" below.

Two sub-packages of `core/` are also imported directly by rpg-api:

- `core/combat` — 1 file, used only for proto enum mapping (`combat.ActionType` and the standard/bonus/reaction/free constants).
- `core/resources` — 1 file (integration helper), aliased as `coreResources` to disambiguate from `rulebooks/dnd5e/resources`.

Both are folded into this doc rather than split into separate component pages.

## Files in core/

| File | Purpose |
|---|---|
| `entity.go` | `Entity` interface, `EntityType` named string |
| `ref.go` | `Ref`, `SourcedRef`, `Source`, `SourceCategory` |
| `action.go` | `Action[T any]` — the activation contract (toolkit-internal) |
| `typed_ref.go` | Generic `TypedRef[T]` for domain-typed identifiers |
| `errors.go` | `ErrNotFound`, `ErrInvalid`, `ErrConflict`, `ErrUnauthorized` |
| `topic.go` | `Topic` (a `string` type used as the event-bus routing key) |
| `chain/` | Generic `Chain[T]` interface and `Stage` type |
| `combat/` | Type aliases for combat constants (see below) |
| `damage/` | `DamageType` constants |
| `effect/`, `features/`, `resources/`, `spells/`, `events/` | Domain type aliases — no executable logic |
| `mock/` | `MockEntity` — gomock implementation |

## Entity

```go
type EntityType string

type Entity interface {
    GetID() string
    GetType() EntityType
}
```

`EntityType` is a distinct named type (`core.EntityType`), not a raw `string`.
This distinction is load-bearing: any mock or test double that returns `string`
from `GetType()` will fail to satisfy `core.Entity` (a previous mock in
`items/validation/basic_validator_test.go` had this drift; resolved per issue
#612).

## Ref — the boundary key

```go
type Ref struct {
    Module string  // e.g., "dnd5e"
    Type   string  // e.g., "features"
    ID     string  // e.g., "rage"
}
```

The routing key for all toolkit content. rpg-api passes these; the toolkit
routes them to implementations. String form: `"dnd5e:features:rage"`.

`SourcedRef` carries provenance:

```go
type SourcedRef struct {
    Ref    *Ref
    Source *Source  // Category + Name (class, background, race, etc.)
}
```

So a UI can later explain "this +2 came from Barbarian Rage." The `Source`
struct has a `Category` (`SourceCategory`) and `Name`.

The dnd5e `refs/` package wraps these constructors into a typed namespace
(`refs.Features.Rage()`, `refs.Conditions.Raging()`, etc.) — see
`refs.md` and `rulebook-dnd5e.md` for that surface.

## Action vs ConditionBehavior — the activation surface

This is the most important architectural distinction in the toolkit, and the
audit's load-bearing correction (`docs/journey/049-rpg-api-toolkit-usage-audit.md`,
Section 3 Claim 1).

`core/action.go` defines the activation contract:

```go
type Action[T any] interface {
    Entity                                                // GetID, GetType
    CanActivate(ctx context.Context, owner Entity, input T) error
    Activate(ctx context.Context, owner Entity, input T) error
}
```

**Features implement `core.Action[T]`.** Example: `Rage` at
`rulebooks/dnd5e/features/rage.go` declares `// CanActivate implements
core.Action[FeatureInput]` and `// Activate implements core.Action[FeatureInput]`
on its method receivers.

**Conditions do NOT implement `core.Action`.** They implement a separate
interface in the dnd5e events package:

```go
// rulebooks/dnd5e/events/events.go (around line 85)
type ConditionBehavior interface {
    IsApplied() bool
    Apply(ctx context.Context, bus events.EventBus) error
    Remove(ctx context.Context, bus events.EventBus) error
    ToJSON() (json.RawMessage, error)
}
```

`RagingCondition` at `rulebooks/dnd5e/conditions/raging.go` asserts this with
`var _ dnd5eEvents.ConditionBehavior = (*RagingCondition)(nil)` and implements
Apply/Remove (subscribing handlers to the bus) — never CanActivate/Activate.

**Mental model:**

- **`core.Action[T]`** is the *activation* half — "thing a player or DM triggers" (Rage feature, Strike, Dodge as combat ability). Lives in `core/`.
- **`dnd5eEvents.ConditionBehavior`** is the *passive/listener* half — "thing that subscribes to the bus and modifies chains while applied" (RagingCondition, Defense fighting style, Unconscious). Lives in `rulebooks/dnd5e/events/`.

A feature can both implement Action **and** apply a Condition as part of its
Activate flow. Rage is the canonical case: `Rage.Activate` (an Action)
constructs and applies a `RagingCondition` (a ConditionBehavior). They are
**related but distinct interfaces** — not "the same thing under the hood."

The toolkit also defines a generic `events.BusEffect` interface (in the
top-level events package) with the same Apply/Remove/IsApplied shape. Inside
the dnd5e rulebook, `ConditionBehavior` is the concrete cousin — it adds
`ToJSON` for the per-condition serialization pattern documented in the
toolkit-root CLAUDE.md.

## core/combat — proto-enum mapping for action types

`core/combat` is types-only; no executable logic. rpg-api's converter file uses
it to map proto enums to/from toolkit constants:

```go
type ActionType string
const (
    ActionStandard ActionType = "action"
    ActionBonus    ActionType = "bonus_action"
    ActionReaction ActionType = "reaction"
    ActionFree     ActionType = "free"
    ActionMovement ActionType = "movement"
)
```

This is exactly the boundary pattern rpg-api should use: rpg-api maps proto
enums to typed toolkit constants, the toolkit owns what each constant means.
The same package also defines `AttackType`, `WeaponProperty`, and `ArmorType`
constants for similar enum-mapping purposes; rpg-api currently only consumes
`ActionType`.

## core/resources — the ResourceAccessor contract

`core/resources` defines the minimal interface a feature needs to consume from
its owner's resource pool:

```go
type ResourceAccessor interface {
    IsResourceAvailable(key ResourceKey) bool
    UseResource(key ResourceKey, amount int) error
}

type ResourceKey string  // e.g., "rage_uses"
type ResetType string    // ResetShortRest, ResetLongRest, ResetDawn, ResetDusk, ResetNever, ResetManual
```

A `Character` implements `ResourceAccessor`; `Rage.CanActivate` type-asserts
its owner to `coreResources.ResourceAccessor` and asks `IsResourceAvailable`.
That pattern — feature owns the rule, character owns the resource pool — is
how the toolkit keeps features decoupled from concrete character types.

rpg-api imports `core/resources` from one integration-test helper
(`internal/integration/encounter/helpers.go`) under the alias `coreResources`
to disambiguate from `rulebooks/dnd5e/resources`. Production code reaches the
resources surface through the dnd5e package.

## Sub-packages without executable logic

`core/chain`, `core/damage`, `core/effect`, `core/features`, `core/spells`,
`core/events` — all define type aliases or constants that callers use. None
have executable methods beyond value types. None have test files. Risk is low
(no logic), but type changes here will only be caught when a downstream module
fails its tests.

`core/chain` is the exception in importance: it defines the generic
`Chain[T any]` interface and the `Stage` type. The events package's
`StagedChain[T]` implements `chain.Chain[T]`. See `events.md` for how the
chain pattern is realized.

## Known gaps

- The sub-packages (`chain/`, `combat/`, `damage/`, etc.) have no test files. Changes to these types will not be caught by CI in the `core` module itself — only when downstream consumers fail.
- No doc on when to use `rpgerr` (toolkit's error-accumulation package) vs `fmt.Errorf`. The answer: use `rpgerr` when the error will be accumulated (multiple validation failures) or when RPG-domain context (entity type, ref, source) adds value for debugging. Use `fmt.Errorf` for simple wrapping.

## Verification

```sh
# Action interface and Rage's implementation
grep -n 'type Action\[T any\] interface' /home/kirk/personal/rpg-toolkit/core/action.go
grep -n 'core\.Action\[FeatureInput\]\|CanActivate\|Activate implements' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/features/rage.go

# ConditionBehavior interface and Raging's implementation
grep -n 'type ConditionBehavior interface' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/events/events.go
grep -n 'ConditionBehavior\|func .* Apply\|func .* Remove' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/conditions/raging.go

# rpg-api's core surface
grep -rln '"github.com/KirkDiggler/rpg-toolkit/core"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 8
grep -rln '"github.com/KirkDiggler/rpg-toolkit/core/combat"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 1
grep -rln '"github.com/KirkDiggler/rpg-toolkit/core/resources"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 1
```
