# RPG Toolkit Architecture

## Overview
The RPG Toolkit uses a traditional modular architecture with clear separation of concerns. Each module has a specific responsibility and communicates with others through well-defined interfaces and events.

## Core Principles
1. **Composition over Inheritance** - Go's interface-based design
2. **Event-Driven Communication** - Modules communicate via events, not direct coupling
3. **Storage Agnostic** - All modules work with interfaces, not concrete implementations
4. **Game System Agnostic** - Core mechanics work for any RPG system

## Module Organization

```
rpg-toolkit/
├── core/                    # Foundation types (Entity, Error)
├── events/                  # Event bus and base event types
├── storage/                 # Storage implementations
│   ├── memory/             # In-memory storage
│   ├── postgres/           # PostgreSQL storage
│   └── redis/              # Redis storage
├── world/                   # Physical world representation
│   ├── locations/          # Rooms, dungeons, areas
│   ├── maps/               # Map data and navigation
│   └── environment/        # Environmental effects
├── entities/                # Living beings
│   ├── characters/         # Player characters
│   ├── monsters/           # NPCs and creatures
│   └── ai/                 # AI behaviors
├── items/                   # Game objects
│   ├── equipment/          # Weapons, armor
│   ├── consumables/        # Potions, scrolls
│   └── treasures/          # Loot, currency
├── mechanics/               # Game mechanics
│   ├── conditions/         # Status effects
│   ├── combat/             # Combat system
│   ├── magic/              # Spell system
│   ├── abilities/          # Skills and abilities
│   └── dice/               # Dice rolling
├── systems/                 # Higher-level systems
│   ├── quests/             # Quest management
│   ├── dialogue/           # Conversation trees
│   ├── crafting/           # Item creation
│   └── economy/            # Trading, shops
└── games/                   # Game-specific rules
    ├── dnd5e/              # D&D 5th Edition
    ├── pathfinder/         # Pathfinder rules
    └── custom/             # Custom game systems
```

## Module Dependencies

### Dependency Rules
1. Modules can only depend on modules at the same level or below
2. Core modules (core, events) have no dependencies
3. Game-specific modules can depend on any mechanics/systems
4. No circular dependencies allowed

### Dependency Hierarchy
```
Level 0: core/
Level 1: events/ (depends on core)
Level 2: storage/* (depends on core)
Level 3: world/, entities/, items/ (depend on core, events)
Level 4: mechanics/* (depend on core, events, and level 3 modules as needed)
Level 5: systems/* (depend on all lower levels as needed)
Level 6: games/* (depend on all lower levels as needed)
```

## Event-Driven Architecture

### Example: Poison Damage
1. **Dungeon Room** emits `environment_entered` event with poison gas effect
2. **Conditions Module** listens and applies poisoned condition
3. **Combat Module** listens to condition applied and deals damage
4. **Quest Module** might listen to track "survive the poison chamber"

### Benefits
- Modules remain decoupled
- Easy to add new features without modifying existing code
- Game rules can override default behaviors by intercepting events

## Storage Patterns

All modules use storage interfaces:

```go
type Storage interface {
    Save(ctx context.Context, entity Entity) error
    Load(ctx context.Context, id string) (Entity, error)
    Delete(ctx context.Context, id string) error
}
```

This allows:
- In-memory storage for testing
- PostgreSQL for production
- Redis for caching
- Easy migration between storage systems

## Example: Combat with Conditions

```go
// Character enters combat with a rage ability
character := &Character{ID: "char-1"}

// Combat module publishes combat_started event
eventBus.Publish(CombatStartedEvent{Entity: character})

// Abilities module listens and activates rage
// This publishes ability_activated event
eventBus.Publish(AbilityActivatedEvent{
    Ability: "rage",
    Source: character,
})

// Conditions module listens and applies rage condition
condition := NewRageCondition(character)
manager.Add(character.ID, condition)

// Combat module listens to condition changes
// and applies damage modifiers from rage
```

## Benefits of This Architecture

1. **Clear Organization** - Every concept has an obvious home
2. **Extensibility** - Add new modules without touching existing code
3. **Testability** - Mock interfaces and test modules in isolation
4. **Reusability** - Use subsets of modules for different projects
5. **Game Agnostic** - Core modules work for any RPG system
6. **Performance** - Event bus can be optimized without changing module code

## Migration from Other Architectures

If we need to migrate to ECS or another architecture later:
1. Keep module interfaces stable
2. Implement ECS components behind existing interfaces
3. Gradually migrate internal implementations
4. Event system remains the same, maintaining compatibility