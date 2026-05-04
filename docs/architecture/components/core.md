---
name: core module
description: Fundamental interfaces and types that every other module depends on
updated: 2026-05-02
confidence: high — verified by reading all files in core/
---

# core module

**Path:** `core/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/core`
**Grade:** A-

The foundation. Every other toolkit module imports `core`. It defines the minimum shared vocabulary for game objects, identifiers, errors, and actions.

## Files

| File | Purpose |
|---|---|
| `entity.go` | `Entity` interface, `EntityType` named string type |
| `ref.go` | `Ref`, `TypedRef`, `SourcedRef`, `Source`, `SourceCategory` |
| `action.go` | `Action` interface — activatable game actions |
| `typed_ref.go` | Generic `TypedRef[T]` for domain-typed identifiers |
| `errors.go` | `ErrNotFound`, `ErrInvalid`, `ErrConflict`, `ErrUnauthorized` |
| `topic.go` | `Topic` interface — the key type for event bus subscriptions |
| `generate.go` | `//go:generate` for mocks |
| `chain/` | `Chain`, `ChainResult` — modifier pipeline types |
| `combat/` | Combat-domain type aliases |
| `damage/` | `DamageType` constants |
| `effect/` | `Effect` type |
| `features/` | `Feature` type alias |
| `resources/` | `Resource` type alias |
| `spells/` | Spell type aliases |
| `events/` | Event type aliases |
| `mock/` | `MockEntity` — gomock implementation |

## Key types

### Entity
```go
type EntityType string

type Entity interface {
    GetID() string
    GetType() EntityType
}
```

`EntityType` is a distinct named type (`core.EntityType`), not a raw `string`. This distinction is load-bearing: the `items/validation` test compile failure (`items/validation/basic_validator_test.go:27`) is caused by a mock that returns `string` where `EntityType` is required.

### Ref
```go
type Ref struct {
    Module string  // e.g., "dnd5e"
    Type   string  // e.g., "features"
    Value  string  // e.g., "rage"
}
```

The routing key for all toolkit content. rpg-api passes these; toolkit routes them to implementations. String form: `"dnd5e:features:rage"`.

### SourcedRef
```go
type SourcedRef struct {
    Ref
    Source *Source  // Category + Name (class, background, race, etc.)
    Label  string   // Human-readable display name for breakdowns
}
```

Carries provenance through modifier chains so UI can explain "this +2 came from Barbarian Rage."

## Sub-packages (types-only, no executable logic)

`core/chain`, `core/combat`, `core/damage`, `core/effect`, `core/features`, `core/resources`, `core/spells`, `core/events` — all define type aliases or constants that callers use. None have executable methods beyond value types. None have test files. Risk is low (no logic), but type changes here will only be caught when a downstream module fails its tests.

## Known gaps

- The sub-packages (`chain/`, `combat/`, `damage/`, etc.) have no test files. Changes to these types will not be caught by CI in the `core` module itself — only when downstream consumers fail.
- No doc on when to use `rpgerr` vs `fmt.Errorf`. The answer: use `rpgerr` when the error will be accumulated (multiple validation failures) or when RPG-domain context (entity type, ref, source) adds value for debugging. Use `fmt.Errorf` for simple wrapping.
