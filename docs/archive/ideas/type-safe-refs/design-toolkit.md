# Design: Toolkit Changes for Type-Safe Damage Sources

## Overview

Map existing `DamageSourceType` string constants to the new proto `SourceRef` structure.

## Current State

```go
// rulebooks/dnd5e/combat/attack.go
type DamageSourceType string

const (
    DamageSourceWeapon            DamageSourceType = "weapon"
    DamageSourceAbility           DamageSourceType = "ability"
    DamageSourceRage              DamageSourceType = "dnd5e:conditions:raging"
    DamageSourceSneakAttack       DamageSourceType = "sneak_attack"
    DamageSourceDivineSmite       DamageSourceType = "divine_smite"
    DamageSourceBrutalCritical    DamageSourceType = "dnd5e:conditions:brutal_critical"
    DamageSourceDueling           DamageSourceType = "dnd5e:conditions:fighting_style_dueling"
    DamageSourceTwoWeaponFighting DamageSourceType = "dnd5e:conditions:fighting_style_two_weapon_fighting"
)

type DamageComponent struct {
    Source            DamageSourceType
    OriginalDiceRolls []int
    FinalDiceRolls    []int
    // ...
}
```

## Target State

### Option A: Keep DamageSourceType, Add Conversion

Minimal change - keep existing type, add conversion for proto mapping:

```go
// rulebooks/dnd5e/combat/proto_conversion.go

import (
    v1alpha1 "github.com/KirkDiggler/rpg-api-protos/gen/go/dnd5e/api/v1alpha1"
)

// ToProtoSourceRef converts toolkit DamageSourceType to proto SourceRef
func (d DamageSourceType) ToProtoSourceRef(weapon v1alpha1.Weapon, ability v1alpha1.Ability) *v1alpha1.SourceRef {
    switch d {
    case DamageSourceWeapon:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Weapon{Weapon: weapon},
        }
    case DamageSourceAbility:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Ability{Ability: ability},
        }
    case DamageSourceRage:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_RAGING},
        }
    case DamageSourceSneakAttack:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_SNEAK_ATTACK},
        }
    case DamageSourceDivineSmite:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_DIVINE_SMITE},
        }
    case DamageSourceBrutalCritical:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_BRUTAL_CRITICAL},
        }
    case DamageSourceDueling:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_FIGHTING_STYLE_DUELING},
        }
    case DamageSourceTwoWeaponFighting:
        return &v1alpha1.SourceRef{
            Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_FIGHTING_STYLE_TWO_WEAPON_FIGHTING},
        }
    default:
        return nil
    }
}
```

**Note:** Weapon and Ability sources need the specific enum value passed in, since `DamageSourceWeapon` doesn't know WHICH weapon.

### Option B: Richer DamageComponent

Carry more information in DamageComponent itself:

```go
type DamageComponent struct {
    Source            DamageSourceType
    SourceWeapon      *v1alpha1.Weapon    // Set when Source == DamageSourceWeapon
    SourceAbility     *v1alpha1.Ability   // Set when Source == DamageSourceAbility
    OriginalDiceRolls []int
    FinalDiceRolls    []int
    // ...
}
```

Then conversion is straightforward:

```go
func (d *DamageComponent) ToProtoSourceRef() *v1alpha1.SourceRef {
    switch d.Source {
    case DamageSourceWeapon:
        if d.SourceWeapon != nil {
            return &v1alpha1.SourceRef{
                Source: &v1alpha1.SourceRef_Weapon{Weapon: *d.SourceWeapon},
            }
        }
    case DamageSourceAbility:
        if d.SourceAbility != nil {
            return &v1alpha1.SourceRef{
                Source: &v1alpha1.SourceRef_Ability{Ability: *d.SourceAbility},
            }
        }
    // ... condition cases don't need extra fields
    }
    return nil
}
```

## Recommendation

**Option B** is cleaner - the DamageComponent should know what weapon/ability it represents. This information exists at creation time (when building weapon damage, we know it's a longsword).

## Where Conversion Happens

Conversion to proto happens in **rpg-api**, not toolkit. Toolkit stays proto-agnostic.

```go
// rpg-api: internal/orchestrators/encounter/converters.go

func toProtoDamageComponent(tc *combat.DamageComponent) *v1alpha1.DamageComponent {
    return &v1alpha1.DamageComponent{
        SourceRef:         tc.ToProtoSourceRef(),
        OriginalDiceRolls: tc.OriginalDiceRolls,
        FinalDiceRolls:    tc.FinalDiceRolls,
        // ...
    }
}
```

## File Summary

| File | Changes |
|------|---------|
| rulebooks/dnd5e/combat/attack.go | Add SourceWeapon, SourceAbility to DamageComponent |
| rulebooks/dnd5e/combat/attack.go | Add ToProtoSourceRef() method |
| (rpg-api) converters.go | Use ToProtoSourceRef() when building proto response |

## Testing

```go
func TestDamageComponent_ToProtoSourceRef(t *testing.T) {
    tests := []struct {
        name     string
        comp     DamageComponent
        expected *v1alpha1.SourceRef
    }{
        {
            name: "weapon source",
            comp: DamageComponent{
                Source:       DamageSourceWeapon,
                SourceWeapon: ptr(v1alpha1.Weapon_WEAPON_LONGSWORD),
            },
            expected: &v1alpha1.SourceRef{
                Source: &v1alpha1.SourceRef_Weapon{Weapon: v1alpha1.Weapon_WEAPON_LONGSWORD},
            },
        },
        {
            name: "rage condition",
            comp: DamageComponent{
                Source: DamageSourceRage,
            },
            expected: &v1alpha1.SourceRef{
                Source: &v1alpha1.SourceRef_Condition{Condition: v1alpha1.ConditionId_CONDITION_ID_RAGING},
            },
        },
    }
    // ...
}
```
