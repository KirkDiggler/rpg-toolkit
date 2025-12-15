# AC System Design Notes

Captured from brainstorming session on 2025-12-06.

## Issue Dependency Chain

```
#348 EquipmentSlots on Character (prerequisite)
    ↓
#397 AC Calculation System
    ↓
#354 Monk Unarmored Defense
#360 Fighting Style: Defense
```

## Related Issues Created

- **#401** - Refactor DamageComponent to separate Type and Source (cleanup to match AC pattern)

## Key Design Decisions

### 1. Character is Self-Contained

The game server just does `AddCharacter(char)`. Character has everything:
- `inventory []InventoryItem` - items owned
- `equipmentSlots EquipmentSlots` - which items equipped where
- `conditions []ConditionBehavior` - active conditions
- `EffectiveAC(ctx)` - calculates AC using all the above

No separate gamectx lookup needed for armor. Character resolves its own equipment.

### 2. Composition Over Fake Inheritance

Instead of type assertions scattered everywhere, use `EquippedItem` wrapper:

```go
type EquippedItem struct {
    Item equipment.Equipment  // Composition, not interface embedding
}

func (e *EquippedItem) AsArmor() *armor.Armor
func (e *EquippedItem) AsWeapon() *weapons.Weapon
```

### 3. Clean Type/Source Separation for AC

**This is critical.** Don't repeat the sloppiness in DamageComponent.

- `Type` = category (ACSourceType: base, armor, shield, ability, feature, spell, item, condition)
- `Source` = specific `*core.Ref` (e.g., `dnd5e:armor:chain_mail`)

```go
type ACSourceType string

const (
    ACSourceBase      ACSourceType = "base"
    ACSourceArmor     ACSourceType = "armor"
    ACSourceShield    ACSourceType = "shield"
    ACSourceAbility   ACSourceType = "ability"
    ACSourceFeature   ACSourceType = "feature"   // Unarmored Defense, Fighting Styles
    ACSourceSpell     ACSourceType = "spell"
    ACSourceItem      ACSourceType = "item"      // Magic items
    ACSourceCondition ACSourceType = "condition" // Buffs/debuffs
)

type ACComponent struct {
    Type   ACSourceType  // Category
    Source *core.Ref     // Specific source ref
    Value  int           // Can be negative for debuffs
}

type ACBreakdown struct {
    Total      int
    Components []ACComponent
}
```

### 4. AC Calculation Flow

1. `char.EffectiveAC(ctx)` creates `ACChainEvent` with base components
2. Character populates: base 10, armor AC, shield +2, DEX mod (capped by armor)
3. Fires through chain on event bus
4. Subscribed conditions add their components (Unarmored Defense, Fighting Style, etc.)
5. Returns `ACBreakdown` with total + full component list

### 5. UI Transparency

The breakdown enables UI to show exactly where AC comes from:
- Chain Mail: 16
- Shield: +2
- Defense Fighting Style: +1
- Total: 19

## Implementation Order

### Phase 1: #348 EquipmentSlots

1. `EquipmentSlots` struct (MainHand, OffHand, Armor, Shield, etc.)
2. `EquippedItem` wrapper with `AsArmor()`, `AsWeapon()` helpers
3. `Character.equipmentSlots` field
4. `Data.EquipmentSlots` for persistence
5. `GetEquippedSlot(slot)` - resolves slot → inventory → EquippedItem
6. `EquipItem(slot, itemID)` / `UnequipItem(slot)` methods

### Phase 2: #397 AC Calculation

1. `ACSourceType` constants
2. `ACComponent` struct (Type, Source, Value)
3. `ACBreakdown` struct (Total, Components)
4. `ACChainEvent` for chain-based modifiers
5. `Character.EffectiveAC(ctx)` method
6. Wire up existing conditions (UnarmoredDefense, FightingStyle) to add components

## Notes on Existing Code

### rpg-api Has Equipment Slots in Redis (Premature)

rpg-api implemented `EquipmentSlots` storage separately in Redis. After #348:
- rpg-api should migrate to store/load as part of character.Data
- Separate Redis storage can be deprecated

### DamageComponent Has Sloppiness (#401)

`DamageSourceType` mixes categories and refs:
```go
DamageSourceWeapon = "weapon"              // Category
DamageSourceRage   = "dnd5e:conditions:raging"  // Full ref!
```

#401 tracks cleaning this up to match the AC pattern.

### gamectx.CharacterWeapons Exists

Pattern already exists for weapons - `CharacterWeapons` with `MainHand()`, `OffHand()`, etc.
But we decided Character should be self-contained, so no need for `CharacterArmor` in gamectx.
