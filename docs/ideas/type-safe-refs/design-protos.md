# Design: Proto Changes for Type-Safe Damage Sources

## Overview

Proto changes to replace loose string `DamageComponent.source` with type-safe `SourceRef` using oneof pattern.

## Changes to dnd5e/api/v1alpha1/enums.proto

Add ConditionId enum:

```protobuf
// ConditionId identifies active conditions that can modify damage
// Maps to toolkit's DamageSourceType constants
enum ConditionId {
  CONDITION_ID_UNSPECIFIED = 0;
  CONDITION_ID_RAGING = 1;
  CONDITION_ID_BRUTAL_CRITICAL = 2;
  CONDITION_ID_FIGHTING_STYLE_DUELING = 3;
  CONDITION_ID_FIGHTING_STYLE_TWO_WEAPON_FIGHTING = 4;
  CONDITION_ID_SNEAK_ATTACK = 5;
  CONDITION_ID_DIVINE_SMITE = 6;
}
```

**Note:** Only conditions with concrete damage source use cases. Add more as needed.

## Changes to dnd5e/api/v1alpha1/common.proto

Add SourceRef message:

```protobuf
// SourceRef identifies the source of damage, effects, or other game mechanics.
// Uses oneof to support different source categories while reusing existing enums.
message SourceRef {
  oneof source {
    // Damage from a weapon
    Weapon weapon = 1;

    // Damage from ability modifier (STR, DEX, etc.)
    Ability ability = 2;

    // Damage from an active condition
    ConditionId condition = 3;
  }
}
```

## Changes to dnd5e/api/v1alpha1/encounter.proto

Update DamageComponent:

```protobuf
// DamageComponent represents damage from a single source
message DamageComponent {
  // DEPRECATED: Use source_ref instead
  // string source = 1 [deprecated = true];

  // Type-safe source reference
  SourceRef source_ref = 8;

  repeated int32 original_dice_rolls = 2;
  repeated int32 final_dice_rolls = 3;
  repeated RerollEvent rerolls = 4;
  int32 flat_bonus = 5;
  DamageType damage_type = 6;
  bool is_critical = 7;
}
```

**Migration strategy:**
- Add `source_ref` as new field (field number 8)
- Deprecate `source` string field
- Toolkit populates both during transition
- Remove `source` in future version

## TypeScript Consumption

Generated types enable clean discriminated union pattern:

```typescript
import { SourceRef, Weapon, Ability, ConditionId } from './gen/dnd5e/api/v1alpha1';

// Type guard functions (generated or manual)
function hasWeapon(ref: SourceRef): ref is SourceRef & { weapon: Weapon } {
  return ref.weapon !== undefined;
}

// Component usage
const DamageSourceBadge: React.FC<{ source: SourceRef }> = ({ source }) => {
  if (source.weapon !== undefined) {
    return <WeaponBadge weapon={source.weapon} />;
  }
  if (source.ability !== undefined) {
    return <AbilityBadge ability={source.ability} />;
  }
  if (source.condition !== undefined) {
    return <ConditionBadge condition={source.condition} />;
  }
  return <span>Unknown source</span>;
};
```

## Enum Display Maps

```typescript
// Display name mappings for UI
const CONDITION_DISPLAY: Record<ConditionId, string> = {
  [ConditionId.CONDITION_ID_UNSPECIFIED]: 'Unknown',
  [ConditionId.CONDITION_ID_RAGING]: 'Rage',
  [ConditionId.CONDITION_ID_BRUTAL_CRITICAL]: 'Brutal Critical',
  [ConditionId.CONDITION_ID_FIGHTING_STYLE_DUELING]: 'Dueling',
  [ConditionId.CONDITION_ID_FIGHTING_STYLE_TWO_WEAPON_FIGHTING]: 'Two-Weapon Fighting',
  [ConditionId.CONDITION_ID_SNEAK_ATTACK]: 'Sneak Attack',
  [ConditionId.CONDITION_ID_DIVINE_SMITE]: 'Divine Smite',
};
```

## Toolkit Mapping

Toolkit's `DamageSourceType` constants map to `SourceRef`:

| Toolkit Constant | SourceRef |
|------------------|-----------|
| `DamageSourceWeapon` | `{weapon: <specific weapon>}` |
| `DamageSourceAbility` | `{ability: <STR/DEX/etc>}` |
| `DamageSourceRage` | `{condition: CONDITION_ID_RAGING}` |
| `DamageSourceSneakAttack` | `{condition: CONDITION_ID_SNEAK_ATTACK}` |
| `DamageSourceDivineSmite` | `{condition: CONDITION_ID_DIVINE_SMITE}` |
| `DamageSourceBrutalCritical` | `{condition: CONDITION_ID_BRUTAL_CRITICAL}` |
| `DamageSourceDueling` | `{condition: CONDITION_ID_FIGHTING_STYLE_DUELING}` |
| `DamageSourceTwoWeaponFighting` | `{condition: CONDITION_ID_FIGHTING_STYLE_TWO_WEAPON_FIGHTING}` |

## File Summary

| File | Changes |
|------|---------|
| enums.proto | Add ConditionId enum |
| common.proto | Add SourceRef message with oneof |
| encounter.proto | Add source_ref to DamageComponent, deprecate source string |

## Future Extensions

When use cases arise, add to the oneof:

```protobuf
message SourceRef {
  oneof source {
    Weapon weapon = 1;
    Ability ability = 2;
    ConditionId condition = 3;
    Spell spell = 4;        // Future: spell damage
    // Item item = 5;       // Future: magic item effects
  }
}
```

Each addition reuses existing enums where possible.
