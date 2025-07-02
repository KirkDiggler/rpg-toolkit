# RPG Toolkit

A modular Go toolkit for building RPG game mechanics. Build once, use everywhere - from Discord bots to game servers.

## Vision

RPG Toolkit provides clean, reusable components for RPG mechanics that work across any platform. Our event-driven architecture enables flexible game systems without tight coupling.

## Architecture

### Event-Driven Design

After building a D&D Discord bot, we learned that features need to compose without knowing about each other. The toolkit uses an event system where:

- **Core mechanics emit events**: Combat actions, status changes, dice rolls
- **Features listen and modify**: Rage adds damage, sneak attack triggers conditionally  
- **No direct coupling**: New features can be added without changing core code

Example:
```go
// Rage listens for damage calculations
eventBus.On("calculate_damage", func(e Event) {
    if attacker.HasEffect("rage") && weapon.IsMelee() {
        e.AddModifier(Modifier{Source: "rage", Value: 2})
    }
})
```

### Module Structure

```
rpg-toolkit/
├── core/           # Entity interface, common errors
├── events/         # Event bus and base event types  
├── dice/           # Dice rolling mechanics
├── combat/         # Attack resolution, damage calculation
├── conditions/     # Status effects (poisoned, stunned, etc)
├── creatures/      # Characters, monsters, NPCs
└── campaigns/      # Sessions, encounters, progression
```

### Storage Agnostic

The toolkit defines interfaces, not implementations. Use any storage backend:

```go
type Repository interface {
    Save(ctx context.Context, entity Entity) error
    GetByID(ctx context.Context, id string) (*Entity, error)
}
```

## Getting Started

```bash
# Get the core module
go get github.com/KirkDiggler/rpg-toolkit/core

# Get specific systems
go get github.com/KirkDiggler/rpg-toolkit/combat
go get github.com/KirkDiggler/rpg-toolkit/conditions
```

## Example Usage

```go
package main

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/combat"
)

func main() {
    // Create event bus
    bus := events.New()
    
    // Register feature listeners
    rage.RegisterHandlers(bus)
    sneakAttack.RegisterHandlers(bus)
    
    // Combat emits events, features respond
    result := combat.ResolveAttack(attacker, target, weapon, bus)
}
```

## Development Status

🚧 **Under Active Development** 🚧

We're extracting and refining patterns from a production Discord bot. The API will stabilize as we complete the extraction.

### Current Focus
1. Core entity system and interfaces
2. Event bus implementation  
3. Combat mechanics with event integration
4. Condition/effect system

## Design Principles

1. **Event-Driven**: Features compose through events, not inheritance
2. **Storage Agnostic**: Define behavior, not persistence
3. **Game System Flexible**: Core mechanics work for any ruleset
4. **Well Tested**: Comprehensive test coverage
5. **Real-World Proven**: Patterns extracted from production use

## Contributing

This is currently a personal project, but discussions and ideas are welcome in the issues.

## License

MIT

## Acknowledgments

Patterns and learnings extracted from [dnd-bot-discord](https://github.com/KirkDiggler/dnd-bot-discord).