# Type-Safe Refs - Use Cases

Why we need type-safe damage source references.

## Use Case 1: Damage Breakdown Display

### Context

A Barbarian attacks with a Warhammer while raging. The attack hits and deals damage from multiple sources. The UI needs to display a meaningful breakdown.

### Current Problem

`DamageComponent.source` is a string with inconsistent values:

```go
// Toolkit's DamageSourceType constants
DamageSourceWeapon            = "weapon"
DamageSourceAbility           = "ability"
DamageSourceRage              = "dnd5e:conditions:raging"
DamageSourceSneakAttack       = "sneak_attack"
DamageSourceBrutalCritical    = "dnd5e:conditions:brutal_critical"
DamageSourceDueling           = "dnd5e:conditions:fighting_style_dueling"
```

The UI receives:
```json
{
  "components": [
    {"source": "weapon", "total": 8},
    {"source": "ability", "total": 3},
    {"source": "dnd5e:conditions:raging", "total": 2}
  ]
}
```

To display "Warhammer +8, STR +3, Rage +2", the UI must:
1. Parse strings to determine category
2. Handle inconsistent formats ("weapon" vs "dnd5e:conditions:raging")
3. Map to display names with no type safety
4. Hope no typos exist

### Desired State

UI receives structured data:
```typescript
{
  components: [
    {source: {weapon: "WEAPON_WARHAMMER"}, total: 8},
    {source: {ability: "ABILITY_STRENGTH"}, total: 3},
    {source: {condition: "CONDITION_ID_RAGING"}, total: 2}
  ]
}
```

Now the UI can:
1. Switch on the oneof case (weapon/ability/condition)
2. Use type-safe enum display components
3. Get compile-time safety

---

## Use Case 2: Enum Display Component

### Context

The React frontend needs to display enum values as human-readable text throughout the UI - not just for damage sources, but weapons, abilities, conditions, etc.

### Pattern

```typescript
// Generic pattern for any proto enum
const CONDITION_DISPLAY: Record<ConditionId, string> = {
  [ConditionId.CONDITION_ID_UNSPECIFIED]: 'Unknown',
  [ConditionId.CONDITION_ID_RAGING]: 'Rage',
  [ConditionId.CONDITION_ID_BRUTAL_CRITICAL]: 'Brutal Critical',
  [ConditionId.CONDITION_ID_FIGHTING_STYLE_DUELING]: 'Dueling',
  // ...
};

const ConditionDisplay: React.FC<{condition: ConditionId}> = ({condition}) => (
  <span className="condition-badge">
    {CONDITION_DISPLAY[condition] ?? 'Unknown'}
  </span>
);
```

### Why oneof Enables This

With a flat enum, you'd have:
```typescript
// BAD: Flat enum loses category information
enum DamageSource {
  DAMAGE_SOURCE_WEAPON_LONGSWORD = 1,
  DAMAGE_SOURCE_ABILITY_STRENGTH = 2,
  DAMAGE_SOURCE_RAGING = 3,
}

// Must parse enum name to know it's a weapon vs ability vs condition
// Can't reuse existing Weapon enum display component
```

With oneof:
```typescript
// GOOD: Structure tells you category
interface SourceRef {
  weapon?: Weapon;
  ability?: Ability;
  condition?: ConditionId;
}

// Naturally dispatches to existing display components
const DamageSourceBadge: React.FC<{source: SourceRef}> = ({source}) => {
  if (source.weapon !== undefined) {
    return <WeaponDisplay weapon={source.weapon} />;  // Reuse existing!
  }
  if (source.ability !== undefined) {
    return <AbilityDisplay ability={source.ability} />;  // Reuse existing!
  }
  if (source.condition !== undefined) {
    return <ConditionDisplay condition={source.condition} />;
  }
  return <span>Unknown</span>;
};
```

---

## Use Case 3: Different Source Categories

### Context

Damage can come from fundamentally different categories of things:
- **Weapons**: Longsword, Warhammer, Dagger
- **Abilities**: STR modifier, DEX modifier
- **Conditions**: Raging, Brutal Critical, Fighting Style effects
- **Future**: Spells, Items, Environmental effects

### The Problem with Flat Enum

A flat `DamageSource` enum would need to include ALL possibilities:

```protobuf
enum DamageSource {
  DAMAGE_SOURCE_UNSPECIFIED = 0;

  // Weapons (duplicating Weapon enum)
  DAMAGE_SOURCE_LONGSWORD = 1;
  DAMAGE_SOURCE_WARHAMMER = 2;
  // ... 30+ weapons

  // Abilities (duplicating Ability enum)
  DAMAGE_SOURCE_STRENGTH = 100;
  DAMAGE_SOURCE_DEXTERITY = 101;
  // ... 6 abilities

  // Conditions
  DAMAGE_SOURCE_RAGING = 200;
  // ...
}
```

Problems:
1. **Duplication**: Every weapon appears twice (Weapon enum AND DamageSource enum)
2. **Lost type info**: Can't tell `DAMAGE_SOURCE_LONGSWORD` is a weapon without parsing the name
3. **Maintenance burden**: Add a weapon? Update two enums.

### The oneof Solution

```protobuf
message SourceRef {
  oneof source {
    Weapon weapon = 1;      // Reuse existing
    Ability ability = 2;    // Reuse existing
    ConditionId condition = 3;  // New, minimal enum
  }
}
```

Only need to define what's NEW (ConditionId). Weapons and Abilities already exist.

---

## Summary: What Use Cases Reveal

### Core Requirements

1. **UI needs displayable damage breakdowns** - human-readable, not string parsing
2. **Different source categories exist** - weapons, abilities, conditions
3. **Existing enums should be reused** - Weapon, Ability already exist
4. **Type safety throughout** - compile-time checks, no typos

### Why oneof is Right

| Requirement | Flat Enum | oneof |
|-------------|-----------|-------|
| Reuse existing enums | ❌ Duplicates | ✅ References |
| Category information | ❌ Lost in name | ✅ Structural |
| TypeScript unions | ❌ Switch on magic numbers | ✅ Discriminated unions |
| Add new category | ❌ Renumber everything | ✅ Add field |
| Maintenance | ❌ Keep in sync | ✅ Single source of truth |

### Scope

Only implementing what we have concrete use cases for:
- `Weapon` (existing enum)
- `Ability` (existing enum)
- `ConditionId` (new enum with: raging, brutal_critical, fighting styles, sneak_attack, divine_smite)

No spells, items, or homebrew until concrete use cases exist.
