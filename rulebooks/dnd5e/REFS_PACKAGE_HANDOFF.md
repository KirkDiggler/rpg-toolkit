# Handoff: Create refs Package for Discoverable Type-Safe References

## Context

We have PR #378 (`refactor/eliminate-magic-strings`) that converts `Source string` fields to `*core.Ref`. However, this doesn't fully solve the problem:

- Sub-packages still use magic strings inside `core.Ref{}` literals
- No compile-time safety for IDs
- Poor discoverability

## The Goal

Create a `refs` package that provides **namespaced, discoverable, type-safe references**.

Developer experience should be:
```go
import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"

// Type refs.<tab> and see: Features, Conditions, Classes, Weapons, etc.
// Type refs.Features.<tab> and see: Rage, SecondWind, etc.

source := refs.Features.Rage()
condition := refs.Conditions.UnarmoredDefense()
class := refs.Classes.Barbarian()
weapon := refs.Weapons.Longsword()
```

## Design Pattern

Use struct-based namespaces for autocomplete discovery:

```go
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

const Module = "dnd5e"

const (
    TypeFeatures   = "features"
    TypeConditions = "conditions"
    TypeClasses    = "classes"
    // etc.
)

// Features namespace
var Features = featuresNS{}

type featuresNS struct{}

func (featuresNS) Rage() *core.Ref {
    return &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
}

func (featuresNS) SecondWind() *core.Ref {
    return &core.Ref{Module: Module, Type: TypeFeatures, ID: "second_wind"}
}

// Conditions namespace
var Conditions = conditionsNS{}

type conditionsNS struct{}

func (conditionsNS) Raging() *core.Ref {
    return &core.Ref{Module: Module, Type: TypeConditions, ID: "raging"}
}

// etc.
```

## Implementation Plan

### Phase 1: Create refs package
1. Create `rulebooks/dnd5e/refs/` directory
2. Create `module.go` with `Module` and `Type*` constants
3. Create `features.go` with `Features` namespace (Rage, SecondWind)
4. Create `conditions.go` with `Conditions` namespace (Raging, UnarmoredDefense, BrutalCritical, FightingStyle)
5. Create `classes.go` with `Classes` namespace (Barbarian, Fighter, Monk, etc.)
6. Build and test

### Phase 2: Migrate existing code
1. Update `classes/grant.go` to use `refs.Features.Rage()` etc.
2. Update `conditions/factory.go` to use refs
3. Update `conditions/raging.go` to use refs
4. Update `features/rage.go` to use refs
5. Update `character/draft.go` to use refs
6. Run tests after each file

### Phase 3: Add remaining namespaces
1. `refs/races.go` - Human, Elf, Dwarf, etc.
2. `refs/weapons.go` - Longsword, Greataxe, etc.
3. `refs/spells.go` - (when needed)
4. `refs/equipment.go` - (when needed)

### Phase 4: Cleanup
1. Remove `dnd5e/refs.go` (the old helper file)
2. Update any remaining magic strings
3. Update documentation

## Files to Reference

- `rulebooks/dnd5e/conditions/types.go` - has `Type = "conditions"` and condition IDs
- `rulebooks/dnd5e/features/types.go` - has `Type = "features"` and feature IDs
- `rulebooks/dnd5e/classes/classes.go` - has class constants
- `rulebooks/dnd5e/classes/grant.go` - main consumer of refs (grants features/conditions)
- `docs/design/eliminate-magic-strings/audit.md` - full audit of magic strings

## Key Constraint

The `refs` package must be a **leaf package** - it can only import `core`. This ensures all other dnd5e packages can import it without cycles.

## Branch Strategy

Start fresh from main:
```bash
git checkout main
git pull
git checkout -b refactor/refs-package
```

## Success Criteria

1. `refs.Features.Rage()` returns `*core.Ref{Module: "dnd5e", Type: "features", ID: "rage"}`
2. All dnd5e sub-packages can import refs without cycles
3. IDE autocomplete shows namespaces and methods
4. All existing tests pass
5. No more hardcoded `"dnd5e"` strings scattered in code
