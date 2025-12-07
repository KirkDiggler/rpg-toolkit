# Type-Safe Refs - Brainstorm

Exploring approaches to replace loose string damage sources with type-safe references.

## Problem

`DamageComponent.source` in the protos is a plain string:

```protobuf
message DamageComponent {
  string source = 1; // "weapon", "ability", "dnd5e:conditions:raging", etc.
  // ...
}
```

The toolkit uses `DamageSourceType` string constants:

```go
DamageSourceWeapon            DamageSourceType = "weapon"
DamageSourceAbility           DamageSourceType = "ability"
DamageSourceRage              DamageSourceType = "dnd5e:conditions:raging"
DamageSourceSneakAttack       DamageSourceType = "sneak_attack"
DamageSourceBrutalCritical    DamageSourceType = "dnd5e:conditions:brutal_critical"
DamageSourceDueling           DamageSourceType = "dnd5e:conditions:fighting_style_dueling"
DamageSourceTwoWeaponFighting DamageSourceType = "dnd5e:conditions:fighting_style_two_weapon_fighting"
```

Problems:
1. **Inconsistent formats**: Some are simple (`"weapon"`), some are refs (`"dnd5e:conditions:raging"`)
2. **No type safety**: Typos compile fine
3. **UI can't display**: No way to get "Warhammer" or "Rage" from these strings
4. **No category info**: Can't tell if source is a weapon, ability, or condition

## Approach A: Flat Enum

Single enum capturing all damage sources:

```protobuf
enum DamageSource {
  DAMAGE_SOURCE_UNSPECIFIED = 0;

  // Core
  DAMAGE_SOURCE_WEAPON = 1;
  DAMAGE_SOURCE_ABILITY = 2;

  // Features/Conditions
  DAMAGE_SOURCE_RAGE = 10;
  DAMAGE_SOURCE_SNEAK_ATTACK = 11;
  DAMAGE_SOURCE_DIVINE_SMITE = 12;
  DAMAGE_SOURCE_BRUTAL_CRITICAL = 13;

  // Fighting styles
  DAMAGE_SOURCE_DUELING = 20;
  DAMAGE_SOURCE_TWO_WEAPON_FIGHTING = 21;
}
```

### Pros
- Simple, single field
- Direct 1:1 mapping to toolkit constants
- Easy switch statements

### Cons
- **Loses specificity**: `DAMAGE_SOURCE_WEAPON` doesn't tell you WHICH weapon
- **No category info**: Can't tell `DAMAGE_SOURCE_RAGE` is a condition
- **Duplication risk**: If we want specific weapons, we duplicate the Weapon enum
- **Doesn't scale**: Adding homebrew requires proto changes

## Approach B: Structured SourceRef with oneof

Message that mirrors toolkit's `module:type:id` pattern:

```protobuf
message SourceRef {
  oneof source {
    Weapon weapon = 1;       // Existing enum
    Ability ability = 2;     // Existing enum
    ConditionId condition = 3; // New enum
  }
}
```

### Pros
- **Reuses existing enums**: Weapon, Ability already exist
- **Structure = category**: `source.weapon` tells you it's a weapon
- **TypeScript discriminated unions**: Perfect mapping
- **No duplication**: Add weapons to Weapon enum, automatically available as damage sources

### Cons
- More complex message structure
- Requires new ConditionId enum

## React Expert Input

Consulted react-protobuf-expert agent. Key insight:

> The `oneof` naturally handles "this could be different types" - that's its purpose.

TypeScript consumption with oneof:
```typescript
const DamageSourceBadge: React.FC<{source: SourceRef}> = ({source}) => {
  if (source.weapon !== undefined) {
    return <WeaponDisplay weapon={source.weapon} />;
  }
  if (source.ability !== undefined) {
    return <AbilityDisplay ability={source.ability} />;
  }
  if (source.condition !== undefined) {
    return <ConditionDisplay condition={source.condition} />;
  }
  return <span>Unknown</span>;
};
```

With flat enum, you'd need to parse enum names or maintain parallel category mappings.

## Decision: Approach B (oneof)

**Reasoning:**
1. The toolkit's existing pattern uses different categories (weapon, ability, condition)
2. We already have Weapon and Ability enums - reuse them
3. oneof provides structural category information
4. TypeScript discriminated unions are the idiomatic consumption pattern
5. Only need to define what's NEW (ConditionId)

## What About Homebrew?

Considered adding a string escape hatch:

```protobuf
message SourceRef {
  oneof source {
    Weapon weapon = 1;
    Ability ability = 2;
    ConditionId condition = 3;
    string custom = 10;  // Escape hatch
  }
}
```

**Decision: Don't add it.**

Adding it now would bias us toward string-based solutions when a more elegant typed approach might exist. We'll add extensibility when we have a concrete homebrew use case, not before.

## Minimal ConditionId Enum

Only conditions that appear in toolkit's `DamageSourceType`:

```protobuf
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

Sneak Attack and Divine Smite are being added as conditions in the toolkit now.

## Open Questions (Resolved)

### Q: Do we need FeatureId separate from ConditionId?

**Answer: No, not yet.**

Features (like Rage) *activate* conditions (like Raging). Damage comes from the *condition*, not the feature. The ActionEconomy idea tracks which feature was activated; the damage system tracks which condition is modifying damage.

If we need to track "damage from this feature" separately from "damage from this condition", we'll add FeatureId then.

### Q: What about spell damage?

**Answer: Not yet.**

When we implement spell damage, we'll add `Spell spell = 4;` to the oneof. The Spell enum already exists. But we don't have spell damage use cases today.

## Next Steps

- Design proto changes (design-protos.md)
- Create implementation issues
- Implement in order: protos → toolkit → web
