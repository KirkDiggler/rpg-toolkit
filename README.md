# RPG Toolkit

A modular Go toolkit for building RPG game mechanics. Build once, use everywhere - from Discord bots to game servers.

## Vision

RPG Toolkit provides clean, reusable components for RPG mechanics that work across any platform. Our event-driven architecture enables flexible game systems without tight coupling.

## Core Principles

### Data-Driven Runtime Architecture

**Everything in the toolkit operates via data at runtime.** No compile-time knowledge, no type checking for specific features, no hardcoded game rules. Just data, interfaces, and runtime discovery.

See [Journey 024: Data-Driven Runtime Architecture](docs/journey/024-data-driven-runtime-architecture.md) for the complete philosophy.

## Architecture

### Hybrid Event-Driven Design

After evaluating ECS (Entity Component System), Event Sourcing, and traditional OOP approaches, we chose a hybrid architecture that combines the best of each:

- **Traditional module structure** for clarity and familiarity
- **Event-driven communication** for loose coupling between features
- **Interface-based design** for extensibility without inheritance
- **Data-driven behavior** where everything loads from data at runtime

This gives us the flexibility of ECS and the decoupling of Event Sourcing without their complexity overhead - perfect for turn-based RPG mechanics.

### How It Works

- **Core mechanics emit events**: Combat actions, status changes, dice rolls
- **Features listen and modify**: Rage adds damage, sneak attack triggers conditionally  
- **No direct coupling**: New features can be added without changing core code

Example:
```go
// Rage listens for damage calculations
eventBus.SubscribeFunc(events.EventCalculateDamage, 100, func(ctx context.Context, e events.Event) error {
    if isRaging(e.Source()) {
        e.Context().AddModifier(events.NewModifier(
            "rage",
            events.ModifierDamageBonus,
            events.NewRawValue(2, "rage"),
            100,
        ))
    }
    return nil
})
```

See [ADR-0002](docs/adr/0002-hybrid-architecture.md) for the full architectural decision.

### Module Structure

```
rpg-toolkit/
├── core/           # Entity interface, common errors
├── events/         # Event bus and base event types  
├── dice/           # Cryptographically secure dice rolling
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
go get github.com/KirkDiggler/rpg-toolkit/dice
go get github.com/KirkDiggler/rpg-toolkit/combat
go get github.com/KirkDiggler/rpg-toolkit/conditions
```

## Example Usage

### Dice Rolling

```go
import "github.com/KirkDiggler/rpg-toolkit/dice"

// Simple dice rolls
attackRoll := dice.D20(1).GetValue()      // 1d20
damage := dice.D6(2).GetValue()            // 2d6

// With descriptions
roll := dice.D20(1)
fmt.Printf("Attack: %s\n", roll.GetDescription()) // "+d20[17]=17"

// As event modifiers
event.Context().AddModifier(events.NewModifier(
    "weapon damage",
    events.ModifierDamageBonus,
    dice.D8(1),  // Roll implements ModifierValue
    100,
))
```

### Combat System

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
1. ✅ Core entity system and interfaces
2. ✅ Event bus implementation  
3. ✅ Dice rolling system with ModifierValue interface
4. Combat mechanics with event integration
5. Condition/effect system

## Design Principles

1. **Event-Driven**: Features compose through events, not inheritance
2. **Storage Agnostic**: Define behavior, not persistence
3. **Game System Flexible**: Core mechanics work for any ruleset
4. **Well Tested**: Comprehensive test coverage
5. **Real-World Proven**: Patterns extracted from production use

## Contributing

This is currently a personal project, but discussions and ideas are welcome in the issues.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

### Why GPL?

- **Free for everyone**: Use, modify, and distribute freely
- **Improvements stay open**: Any modifications must be shared back
- **Commercial dual licensing**: Available for proprietary use under separate license (contact for details)

## Acknowledgments

Patterns and learnings extracted from [dnd-bot-discord](https://github.com/KirkDiggler/dnd-bot-discord).