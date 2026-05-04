# Design: Game Server (rpg-api) Changes for Type-Safe Damage Sources

## Overview

Game server changes to convert toolkit's `DamageComponent` to proto `DamageComponent` with the new `SourceRef` field.

**Key principle:** rpg-api stores data and orchestrates. rpg-toolkit handles rules. Conversion happens at the boundary.

## Conversion Layer

### DamageComponent Conversion

```go
// internal/orchestrators/encounter/converters.go

import (
    v1alpha1 "github.com/KirkDiggler/rpg-api-protos/gen/go/dnd5e/api/v1alpha1"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// toProtoDamageComponent converts toolkit DamageComponent to proto
func toProtoDamageComponent(tc *combat.DamageComponent) *v1alpha1.DamageComponent {
    return &v1alpha1.DamageComponent{
        SourceRef:         tc.ToProtoSourceRef(), // Toolkit provides conversion
        OriginalDiceRolls: toInt32Slice(tc.OriginalDiceRolls),
        FinalDiceRolls:    toInt32Slice(tc.FinalDiceRolls),
        Rerolls:           toProtoRerolls(tc.Rerolls),
        FlatBonus:         int32(tc.FlatBonus),
        DamageType:        toProtoDamageType(tc.DamageType),
        IsCritical:        tc.IsCritical,
    }
}

// toProtoDamageBreakdown converts toolkit DamageBreakdown to proto
func toProtoDamageBreakdown(tb *combat.DamageBreakdown) *v1alpha1.DamageBreakdown {
    if tb == nil {
        return nil
    }

    components := make([]*v1alpha1.DamageComponent, len(tb.Components))
    for i, c := range tb.Components {
        components[i] = toProtoDamageComponent(&c)
    }

    return &v1alpha1.DamageBreakdown{
        Components:  components,
        AbilityUsed: tb.AbilityUsed,
        TotalDamage: int32(tb.TotalDamage),
    }
}
```

### AttackResult Conversion

```go
// toProtoAttackResult converts toolkit AttackResult to proto
func toProtoAttackResult(ar *combat.AttackResult) *v1alpha1.AttackResult {
    return &v1alpha1.AttackResult{
        Hit:             ar.Hit,
        Critical:        ar.Critical,
        AttackRoll:      toProtoRollResult(ar.AttackRoll),
        DamageBreakdown: toProtoDamageBreakdown(ar.Breakdown),
        TargetAc:        int32(ar.TargetAC),
    }
}
```

## Migration Strategy

During transition, populate both old and new fields:

```go
func toProtoDamageComponent(tc *combat.DamageComponent) *v1alpha1.DamageComponent {
    proto := &v1alpha1.DamageComponent{
        // NEW: Type-safe source reference
        SourceRef: tc.ToProtoSourceRef(),

        // DEPRECATED: Keep for backwards compatibility during migration
        Source: string(tc.Source),

        // ... other fields
    }
    return proto
}
```

Clients can migrate at their own pace:
1. Old clients read `source` string
2. New clients read `source_ref` oneof
3. After all clients migrate, remove `source` field

## Handler Integration

Handlers don't change - they already call orchestrators which return proto types:

```go
// internal/handlers/dnd5e/v1alpha1/encounter/handler.go

func (h *Handler) Attack(
    ctx context.Context,
    req *v1alpha1.AttackRequest,
) (*v1alpha1.AttackResponse, error) {
    // ... validation ...

    output, err := h.encounterService.Attack(ctx, &encounter.AttackInput{
        EncounterID: req.GetEncounterId(),
        AttackerID:  req.GetAttackerId(),
        TargetID:    req.GetTargetId(),
        WeaponID:    req.GetWeaponId(),
    })
    if err != nil {
        return nil, errors.ToGRPCError(err)
    }

    // Orchestrator already converted to proto types
    return &v1alpha1.AttackResponse{
        Result:       output.AttackResult,  // Already proto type
        CombatState:  output.CombatState,
    }, nil
}
```

## Orchestrator Changes

Orchestrators need to pass weapon/ability context to toolkit:

```go
// internal/orchestrators/encounter/attack.go

func (o *Orchestrator) Attack(
    ctx context.Context,
    input *AttackInput,
) (*AttackOutput, error) {
    // ... load entities ...

    // Get weapon enum for SourceRef
    weaponEnum := toProtoWeapon(weapon.ID)

    // Get ability used (STR or DEX based on weapon/finesse)
    abilityEnum := toProtoAbility(attackAbility)

    // Execute attack
    result, err := o.attackResolver.ResolveAttack(ctx, &combat.AttackInput{
        Attacker:      attacker,
        Target:        target,
        Weapon:        weapon,
        WeaponEnum:    weaponEnum,    // Pass for SourceRef
        AbilityEnum:   abilityEnum,   // Pass for SourceRef
    })
    if err != nil {
        return nil, err
    }

    // Convert to proto
    return &AttackOutput{
        AttackResult: toProtoAttackResult(result),
        CombatState:  toProtoCombatState(encounter.CombatState),
    }, nil
}
```

## Enum Conversion Helpers

```go
// internal/handlers/dnd5e/v1alpha1/encounter/converters.go

func toProtoWeapon(weaponID string) v1alpha1.Weapon {
    // Map toolkit weapon IDs to proto enum
    switch weaponID {
    case "longsword":
        return v1alpha1.Weapon_WEAPON_LONGSWORD
    case "warhammer":
        return v1alpha1.Weapon_WEAPON_WARHAMMER
    case "greataxe":
        return v1alpha1.Weapon_WEAPON_GREATAXE
    // ... other weapons
    default:
        return v1alpha1.Weapon_WEAPON_UNSPECIFIED
    }
}

func toProtoAbility(ability string) v1alpha1.Ability {
    switch ability {
    case "STR", "strength":
        return v1alpha1.Ability_ABILITY_STRENGTH
    case "DEX", "dexterity":
        return v1alpha1.Ability_ABILITY_DEXTERITY
    // ... other abilities
    default:
        return v1alpha1.Ability_ABILITY_UNSPECIFIED
    }
}

func toProtoConditionId(source combat.DamageSourceType) v1alpha1.ConditionId {
    switch source {
    case combat.DamageSourceRage:
        return v1alpha1.ConditionId_CONDITION_ID_RAGING
    case combat.DamageSourceBrutalCritical:
        return v1alpha1.ConditionId_CONDITION_ID_BRUTAL_CRITICAL
    case combat.DamageSourceDueling:
        return v1alpha1.ConditionId_CONDITION_ID_FIGHTING_STYLE_DUELING
    case combat.DamageSourceTwoWeaponFighting:
        return v1alpha1.ConditionId_CONDITION_ID_FIGHTING_STYLE_TWO_WEAPON_FIGHTING
    case combat.DamageSourceSneakAttack:
        return v1alpha1.ConditionId_CONDITION_ID_SNEAK_ATTACK
    case combat.DamageSourceDivineSmite:
        return v1alpha1.ConditionId_CONDITION_ID_DIVINE_SMITE
    default:
        return v1alpha1.ConditionId_CONDITION_ID_UNSPECIFIED
    }
}
```

## File Summary

| Layer | File | Changes |
|-------|------|---------|
| Orchestrator | orchestrators/encounter/attack.go | Pass weapon/ability enums to toolkit |
| Orchestrator | orchestrators/encounter/converters.go | toProtoDamageComponent, toProtoDamageBreakdown |
| Handler | handlers/dnd5e/v1alpha1/encounter/converters.go | toProtoWeapon, toProtoAbility, toProtoConditionId |

## Key Principles

1. **Conversion at boundary** - Toolkit stays proto-agnostic, API handles conversion
2. **Toolkit provides helpers** - `ToProtoSourceRef()` method on DamageComponent
3. **Pass context through** - Weapon/ability enums flow from request to toolkit to response
4. **Backwards compatibility** - Populate both fields during migration
5. **Handlers stay thin** - Orchestrators do the work
