# Game Package

The `game` package provides runtime infrastructure for loading and managing game entities. It bridges the gap between static data structures and active game objects that participate in the event system.

## Purpose

This package handles:
- **Entity Loading**: Converting persistent data into active game objects
- **Infrastructure Wiring**: Connecting entities to event buses and other game systems
- **State Management**: Common patterns for managing entity lifecycle
- **Runtime Context**: Providing consistent context for game operations

## What Belongs Here

✅ **YES**:
- `GameContext[T]` - Generic pattern for loading entities with infrastructure
- Entity lifecycle management (loading, activating, deactivating)
- Common runtime patterns shared across different game systems
- Infrastructure integration (event bus, future systems)

❌ **NO**:
- Game rules (attack rolls, damage calculation, saving throws)
- Combat mechanics (initiative, actions, reactions)
- Character progression (leveling, experience)
- Specific entity implementations (Character, Monster, Item)

## Key Concepts

### GameContext

The `GameContext[T]` pattern provides a consistent way to load any game entity:

```go
// All entities load the same way
character := LoadCharacterFromData(ctx, GameContext[CharacterData]{...})
room := LoadRoomFromData(ctx, GameContext[RoomData]{...})
item := LoadItemFromData(ctx, GameContext[ItemData]{...})
```

This ensures:
- Entities have access to required infrastructure (event bus)
- Loading signatures are consistent across the toolkit
- Future infrastructure can be added without changing every loader

## Design Principles

1. **Rule-Agnostic**: This package knows nothing about specific game rules
2. **Infrastructure-Focused**: Provides plumbing, not game logic
3. **Generic Patterns**: Solutions that work for any game system
4. **Minimal Dependencies**: Only depends on core and events

## Example Usage

```go
// Create game context with infrastructure
gameCtx := game.NewContext(eventBus, characterData)

// Load character (in rulebook package)
character, err := character.LoadFromContext(ctx, gameCtx)

// Character is now wired to event bus and ready for gameplay
```

## Future Considerations

As the toolkit grows, this package might also handle:
- Session management
- State persistence coordination
- System registration (combat tracker, vision system, etc.)
- Performance monitoring

However, these will only be added if they remain rule-agnostic infrastructure concerns.