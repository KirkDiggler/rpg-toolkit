# Where RPG Elements Fit in the Architecture

## Core Game Elements and Their Homes

### Physical World Elements
- **Dungeons, Rooms, Areas** → `world/` module
  - Maps, locations, terrain
  - Spatial relationships
  - Environmental effects
  - Visibility/line of sight

### Game Objects
- **Items, Equipment, Treasures** → `items/` module
  - Weapons, armor, consumables
  - Magic items with effects
  - Inventory management
  - Item properties and modifiers

### Living Entities
- **Characters, Monsters, NPCs** → `entities/` module
  - Stats and attributes
  - Skills and abilities
  - AI behaviors
  - Entity relationships

### Game Mechanics
- **Conditions, Effects, Modifiers** → `mechanics/conditions/`
- **Combat System** → `mechanics/combat/`
- **Magic System** → `mechanics/magic/`
- **Skills/Abilities** → `mechanics/abilities/`

### Game Rules
- **D&D 5e Specific Rules** → `games/dnd5e/`
- **Pathfinder Rules** → `games/pathfinder/`
- **Custom Game Rules** → `games/custom/`

## Example: How a Dungeon Works

```go
// world/location.go
type Location interface {
    GetID() string
    GetType() string // "room", "corridor", "outdoor"
    GetConnections() []string // IDs of connected locations
    GetEntities() []string // Entity IDs present here
    GetEffects() []Effect // Environmental effects
}

// world/dungeon.go
type Dungeon struct {
    ID string
    Name string
    Locations map[string]Location
    ActiveEffects []Effect // Darkness, magical auras, etc
}

// The dungeon can emit events
func (d *Dungeon) OnEntityEnter(entityID string, locationID string) {
    event := NewLocationEvent("entity_entered", locationID, entityID)
    d.eventBus.Publish(event)
    
    // This could trigger:
    // - Traps (combat module listening)
    // - Environmental effects (conditions module)
    // - Monster spawns (entities module)
    // - Quest updates (quests module)
}
```

## Example: Environmental Conditions

```go
// A room with poisonous gas
room := &DungeonRoom{
    ID: "crypt_chamber_1",
    Effects: []Effect{
        PoisonousGas{Damage: 1d6, SaveDC: 12},
    },
}

// When someone enters, the room emits an event
// The conditions module can listen and apply poisoned condition
// The combat module can handle the damage
```

## The Beauty of This Approach

1. **Everything has a clear home** - no ambiguity about where code belongs
2. **Modules communicate via events** - rooms don't need to know about combat
3. **Easy to extend** - add a `vehicles/` module without touching existing code
4. **Game-agnostic core** - the `world/` module works for any game system

## Module Dependencies for World Elements

```
world/
├── depends on: core/, events/
├── emits: location_entered, trap_triggered, environment_changed
└── listens to: entity_moved, time_passed

entities/
├── depends on: core/, events/
├── emits: entity_moved, entity_acted
└── listens to: damage_taken, condition_applied

mechanics/traps/
├── depends on: core/, events/, world/
├── emits: trap_activated, damage_dealt
└── listens to: entity_entered, entity_searched
```

This structure ensures every RPG concept has a logical place while maintaining clean separation of concerns.