# Journey 014: Self-Contained Entity Data Pattern

## Context

As we design the data persistence layer for spatial entities (rooms, characters, monsters), we're exploring whether all game entities should have self-contained data that includes everything needed to reconstruct them, rather than requiring external dependencies.

This journey explores this pattern in the context of our next milestone: **a single dungeon room with monsters and players that can have a combat session**.

## The Problem

Currently, loading a character requires external dependencies:
```go
func LoadCharacterFromData(data Data, raceData *race.Data, classData *class.Data, 
    backgroundData *shared.Background) (*Character, error)
```

This creates challenges:
- Complex loading signatures
- Network/storage requires multiple lookups
- Testing requires setting up dependencies
- Versioning issues when referenced data changes

## The Vision: Self-Contained Entities

```go
// Universal pattern for all entities
func LoadEntityFromData[T any](ctx context.Context, gameCtx GameContext[T]) (Entity, error)

// Clean, consistent usage
room := LoadRoomFromData(ctx, GameContext[RoomData]{...})
character := LoadCharacterFromData(ctx, GameContext[CharacterData]{...})
monster := LoadMonsterFromData(ctx, GameContext[MonsterData]{...})
```

## Milestone Use Case: Combat Session

### Scenario: Combat in a Dungeon Room

1. **Load the Room**
```go
roomData := RoomData{
    ID:     "dungeon-1",
    Width:  20,
    Height: 20,
    Walls:  []WallData{...},      // Self-contained wall definitions
    Traps:  []TrapData{...},       // Self-contained trap mechanics
}
room := LoadRoomFromData(ctx, GameContext[RoomData]{EventBus: bus, Data: roomData})
```

2. **Load the Character**
```go
charData := CharacterData{
    ID:            "hero-1",
    Name:          "Thorin",
    HP:            45,
    AC:            16,
    Attacks:       []AttackData{...},     // Compiled attack bonuses
    Features:      []FeatureData{...},    // Baked-in class/race features
    SpellSlots:    SpellSlotData{...},    // Pre-calculated slots
}
character := LoadCharacterFromData(ctx, GameContext[CharacterData]{EventBus: bus, Data: charData})
```

3. **Load Monsters**
```go
goblinData := MonsterData{
    ID:       "goblin-1",
    Name:     "Goblin",
    HP:       7,
    AC:       15,
    Attacks:  []AttackData{{Name: "Scimitar", Bonus: 4, Damage: "1d6+2"}},
    Features: []FeatureData{{Name: "Nimble Escape", Effect: {...}}},
}
goblin := LoadMonsterFromData(ctx, GameContext[MonsterData]{EventBus: bus, Data: goblinData})
```

4. **Combat Context**
```go
// Different context for different game phases
type CombatContext struct {
    EventBus    events.EventBus
    Room        *Room
    Combatants  []Combatant
    TurnOrder   []string
}

// But entities remain self-contained
character.Attack(combatCtx, targetID, weaponIndex)
// The character has everything it needs to calculate the attack
```

## Design Decisions

### 1. What Goes in Entity Data?

**Include (Mechanical Data):**
- Stats that affect gameplay (HP, AC, abilities)
- Compiled bonuses (attack +7, not "STR mod + proficiency")
- Active effects/conditions
- Resources (spell slots, ki points)

**Reference Only (Display Data):**
- Source IDs (raceID, classID) for UI display
- Flavor text
- Historical choices

### 2. How Do Features Work?

Features are self-contained effects:
```go
type FeatureData struct {
    ID          string
    Name        string
    Type        string  // "passive", "action", "reaction"
    Effect      EffectData {
        Trigger     string  // "on_attack", "on_damaged", etc.
        Modifier    string  // "+1d6 damage", "+2 AC", etc.
        Condition   string  // "if target is surprised"
    }
}
```

### 3. Context Types

Different game phases need different contexts:
```go
// Loading/persistence
type GameContext[T any] struct {
    EventBus events.EventBus
    Data     T
}

// Active combat
type CombatContext struct {
    EventBus   events.EventBus
    Combat     *CombatSession
    Room       *Room
}

// Future: Campaign play
type CampaignContext struct {
    EventBus   events.EventBus
    Campaign   *Campaign
    QuestLog   *QuestLog
}
```

## Benefits for Combat Milestone

1. **Simple Loading**: One function pattern for all entities
2. **Easy Testing**: Create test data without dependencies
3. **Network Ready**: Send one blob per entity
4. **Parallel Loading**: No shared dependencies
5. **Clear Contracts**: Entity data defines capabilities

## Escape Hatches

When we need external data:

1. **Registry Pattern**
```go
type EntityRegistry interface {
    GetFeature(id string) (*Feature, error)
    GetSpell(id string) (*Spell, error)
}

// Optional registry in context
gameCtx := GameContext[CharacterData]{
    Data:     charData,
    Registry: registry, // For complex lookups
}
```

2. **Lazy Loading**
```go
type FeatureData struct {
    ID       string
    Details  *FeatureDetails  // Nil for lazy load
    // Loaded on first use if needed
}
```

## Open Questions

1. **Spells**: Should spell data be embedded or referenced?
   - Embedded: Character has full spell descriptions
   - Referenced: Character has spell IDs, lookup when cast

2. **Items**: How do equipped items affect character data?
   - Option A: Bake bonuses into character stats
   - Option B: Keep items separate, calculate on use

3. **Conditions**: Temporary effects during combat
   - Store in entity data and recalculate each load?
   - Keep separate in combat context?

## Next Steps

1. Update rpg-toolkit #107 to use this pattern
2. Design combat session structure
3. Determine spell/item handling
4. Create ADR once pattern is proven

## Conclusion

The self-contained entity pattern appears to offer significant benefits for our combat milestone while maintaining flexibility for future features. By focusing on "compiled" game data rather than source references, we can create a clean, consistent API that's easy to test and extend.