---
name: items module
description: Item interface definitions — infrastructure only, no implementations
updated: 2026-05-04
confidence: high — verified by reading item.go, go.mod, and items/validation/basic_validator_test.go
---

# items module

**Path:** `items/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/items`
**Grade:** C

Interface definitions for game items. No implementing structs in the base module — those live in `rulebooks/dnd5e/weapons`, `rulebooks/dnd5e/armor`, etc. The base module is intentionally thin. Its tests now compile (issue #612 resolved); the go.mod still carries a `replace` directive (issue #613).

## Files

| File | Purpose |
|---|---|
| `item.go` | `Item`, `EquippableItem`, `WeaponItem`, `ArmorItem`, `ConsumableItem` interfaces |
| `doc.go` | Package documentation |
| `validation/` | `BasicValidator`, `Validator` interface |
| `validation/basic_validator.go` | Validates item fields |
| `validation/basic_validator_test.go` | Tests for `BasicValidator` |
| `validation/edge_cases_test.go` | Tests for edge cases |
| `validation/validator.go` | `Validator` interface |

## go.mod violation (issue #613)

```
replace github.com/KirkDiggler/rpg-toolkit/core => ../core
```

One committed replace directive remaining. The mocks now satisfy `core.Entity`, so `go test ./...` builds and runs from the items directory.

## What items provides

```go
type Item interface {
    core.Entity      // GetID() string, GetType() EntityType
    GetWeight() float64
    GetValue() int
    GetProperties() []string
    IsStackable() bool
    GetMaxStack() int
}

type EquippableItem interface {
    Item
    GetValidSlots() []string
    GetRequiredSlots() []string
    IsAttunable() bool
    RequiresAttunement() bool
}

// WeaponItem, ArmorItem, ConsumableItem follow same pattern
```

The interfaces are well-designed: composable, minimal, no business logic. The implementation gap is in the test layer.
