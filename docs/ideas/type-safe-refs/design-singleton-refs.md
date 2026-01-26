# Design: Singleton Refs for Pointer Identity

## Status: Part 1 & 2 Complete (rpg-toolkit)

**Completed:**
- Part 1: Singleton refs in refs package
- Part 2: `Weapons.ByID()` lookup and `weaponToRef` updated

**Remaining:** Part 3 - rpg-api converters (issue #269)

**Issues:**
- rpg-toolkit: [#412](https://github.com/KirkDiggler/rpg-toolkit/issues/412)
- rpg-api: [#269](https://github.com/KirkDiggler/rpg-api/issues/269)

## Problem

The current refs package allocates a new `*core.Ref` on every call:

```go
func (n abilitiesNS) Strength() *core.Ref { return n.ref("str") }

func (n ns) ref(id core.ID) *core.Ref {
    return &core.Ref{Module: Module, Type: n.t, ID: id}  // New allocation
}
```

This means converters must compare string IDs:

```go
// Current: string comparison
switch id {
case "str":  // magic string, no IDE help
    return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
}

// Or extract ID from ref
switch ref.ID {
case refs.Abilities.Strength().ID:  // allocates just to get string
    return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
}
```

## Solution: Singleton Refs

Return cached singleton pointers instead of allocating.

### Pattern Options Considered

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| A | Store in namespace struct fields | Grouped by namespace | Verbose struct init |
| B | Package-level unexported vars | Explicit, zero runtime cost | More vars to write |
| C | Modify `ns.ref()` to cache | Minimal code change | Runtime map lookup |

### Decision: Option B - Package-level vars

**Rationale:**
- Explicit over implicit - we control exactly which refs are singletons
- Zero runtime overhead - no map lookups, direct pointer return
- Unexported vars - controlled access through methods
- Worth the extra code for predictable performance

### Implementation Pattern

```go
// Package-level singletons (unexported)
var (
    abilityStrength  = &core.Ref{Module: Module, Type: TypeAbilities, ID: "str"}
    abilityDexterity = &core.Ref{Module: Module, Type: TypeAbilities, ID: "dex"}
    // ...
)

func (n abilitiesNS) Strength() *core.Ref  { return abilityStrength }
func (n abilitiesNS) Dexterity() *core.Ref { return abilityDexterity }
```

**Naming convention:** `{type}{Name}` - e.g., `abilityStrength`, `weaponLongsword`, `conditionRaging`

## Benefits

### 1. Pointer Identity Comparison

```go
// Direct pointer comparison - fast and type-safe
if ref == refs.Abilities.Strength() {
    return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
}

// Switch on ref directly
switch ref {
case refs.Abilities.Strength():
    return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
case refs.Abilities.Dexterity():
    return dnd5ev1alpha1.Ability_ABILITY_DEXTERITY
}
```

### 2. Zero Allocations

Same pointer returned every time - no GC pressure.

### 3. IDE Discoverability

Type `refs.Abilities.` and autocomplete shows all options.

### 4. Backward Compatible

Public API unchanged - callers don't need to change anything.

## End-to-End Requirement

For pointer comparison to work, **both ends must use the same singletons**:

### Toolkit Side (creates refs)

Currently mixed - some use refs, some create new:

```go
// abilityToRef - ALREADY uses refs ✅
func abilityToRef(ability abilities.Ability) *core.Ref {
    switch ability {
    case abilities.STR:
        return refs.Abilities.Strength()  // Will be singleton
    }
}

// weaponToRef - creates NEW refs ❌
func weaponToRef(weapon *weapons.Weapon) *core.Ref {
    return &core.Ref{
        Module: refs.Module,
        Type:   refs.TypeWeapons,
        ID:     weapon.ID,  // New allocation
    }
}
```

**Must update to use refs package:**

```go
// weaponToRef - use refs singletons
func weaponToRef(weapon *weapons.Weapon) *core.Ref {
    switch weapon.ID {
    case "longsword":
        return refs.Weapons.Longsword()
    case "greataxe":
        return refs.Weapons.Greataxe()
    // ...
    }
}
```

### API Side (converts refs to proto)

With singletons, converters become simple:

```go
func convertCoreRefToProtoSourceRef(ref *core.Ref) *dnd5ev1alpha1.SourceRef {
    // Switch on ref type first
    switch ref.Type {
    case refs.TypeAbilities:
        return &dnd5ev1alpha1.SourceRef{
            Source: &dnd5ev1alpha1.SourceRef_Ability{
                Ability: abilityRefToProto(ref),
            },
        }
    // ...
    }
}

func abilityRefToProto(ref *core.Ref) dnd5ev1alpha1.Ability {
    switch ref {
    case refs.Abilities.Strength():
        return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
    case refs.Abilities.Dexterity():
        return dnd5ev1alpha1.Ability_ABILITY_DEXTERITY
    // IDE autocomplete, compile-time safety
    }
}
```

## Fallback for Unknown Refs

If toolkit creates a ref that's not a known singleton (e.g., homebrew), fall back to string comparison:

```go
func abilityRefToProto(ref *core.Ref) dnd5ev1alpha1.Ability {
    // Fast path: pointer comparison for known singletons
    switch ref {
    case refs.Abilities.Strength():
        return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
    // ...
    }

    // Slow path: string comparison for unknown refs
    switch ref.ID {
    case "str":
        return dnd5ev1alpha1.Ability_ABILITY_STRENGTH
    // ...
    default:
        return dnd5ev1alpha1.Ability_ABILITY_UNSPECIFIED
    }
}
```

## Implementation Order

1. **rpg-toolkit: refs package** - Convert to singletons (backward compatible)
2. **rpg-toolkit: combat/attack.go** - Update weaponToRef, conditionToRef, etc.
3. **rpg-api: converters** - Update to use pointer comparison

## Files to Change

### rpg-toolkit

| File | Changes |
|------|---------|
| rulebooks/dnd5e/refs/abilities.go | Add singleton vars, update methods |
| rulebooks/dnd5e/refs/weapons.go | Add singleton vars, update methods |
| rulebooks/dnd5e/refs/conditions.go | Add singleton vars, update methods |
| rulebooks/dnd5e/refs/features.go | Add singleton vars, update methods |
| rulebooks/dnd5e/refs/*.go | All namespace files get singletons |
| rulebooks/dnd5e/combat/attack.go | Update weaponToRef, conditionToRef helpers |

### rpg-api

| File | Changes |
|------|---------|
| internal/handlers/dnd5e/v1alpha1/encounter/converters.go | Use pointer comparison in switches |

## Testing

```go
func TestSingletonIdentity(t *testing.T) {
    // Same pointer returned
    ref1 := refs.Abilities.Strength()
    ref2 := refs.Abilities.Strength()
    assert.Same(t, ref1, ref2)  // Pointer equality

    // Different refs are different pointers
    str := refs.Abilities.Strength()
    dex := refs.Abilities.Dexterity()
    assert.NotSame(t, str, dex)
}

func TestConverterPointerComparison(t *testing.T) {
    ref := refs.Abilities.Strength()
    result := abilityRefToProto(ref)
    assert.Equal(t, dnd5ev1alpha1.Ability_ABILITY_STRENGTH, result)
}
```
