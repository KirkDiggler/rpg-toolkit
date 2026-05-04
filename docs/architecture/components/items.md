---
name: items module
description: Item interface definitions — infrastructure only, no implementations, tests broken
updated: 2026-05-02
confidence: high — verified by reading item.go, go.mod, and items/validation/basic_validator_test.go
---

# items module

**Path:** `items/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/items`
**Grade:** D

Interface definitions for game items. No implementing structs in the base module — those live in `rulebooks/dnd5e/weapons`, `rulebooks/dnd5e/armor`, etc. The base module is intentionally thin, but its test layer is broken and its go.mod carries a replace directive.

## Files

| File | Purpose |
|---|---|
| `item.go` | `Item`, `EquippableItem`, `WeaponItem`, `ArmorItem`, `ConsumableItem` interfaces |
| `doc.go` | Package documentation |
| `validation/` | `BasicValidator`, `Validator` interface |
| `validation/basic_validator.go` | Validates item fields |
| `validation/basic_validator_test.go` | **Does not compile** |
| `validation/edge_cases_test.go` | Tests for edge cases |
| `validation/validator.go` | `Validator` interface |

## The compile failure (issue #612)

`items/validation/basic_validator_test.go:27`:
```go
func (m *mockItem) GetType() string { return m.itemType }
```

`core.Entity.GetType()` returns `core.EntityType` (a named type, `core/entity.go:8`):
```go
type EntityType string
type Entity interface {
    GetID() string
    GetType() EntityType  // NOT string
}
```

`mockItem` embeds `mockItem` which implements `Item` which embeds `core.Entity`. The mock's `GetType()` returns `string`, not `core.EntityType`. Go's type system treats these as distinct — the mock does not satisfy the interface.

Running `go test ./...` from the `items/` directory exits with:
```
./validation/basic_validator_test.go:27:6: cannot use type mockItem as type core.Entity in assignment:
	mockItem does not implement core.Entity (wrong type for GetType method)
```

Production code (`item.go`) builds correctly — the issue is only in the test mock.

**Fix:** Change `GetType() string` to `GetType() core.EntityType` in the mock. Also update `itemType string` field to `core.EntityType`. Estimated: 15–30 minutes of work.

## go.mod violation (issue #613)

```
replace github.com/KirkDiggler/rpg-toolkit/core => ../core
```

One committed replace directive. Combined with the broken tests, this module cannot pass CI in its current state.

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
