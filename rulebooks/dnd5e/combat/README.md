# D&D 5e Combat Initiative System

This package implements the D&D 5e initiative system following the RPG Toolkit's established patterns for rich domain objects, event-driven architecture, and persistence.

## Overview

The combat system provides:

- **Initiative Rolling**: Roll 1d20 + DEX modifier for each combatant
- **Turn Order Management**: Sort combatants by initiative (highest first)
- **Tie Breaking**: Handle ties using DEX scores, DM decisions, or re-rolls
- **Round Tracking**: Manage combat rounds and turn progression
- **Event Integration**: Emit events for all combat state changes
- **Persistence**: Save/load combat state using the GameContext pattern

## Architecture

The implementation follows the toolkit's domain-rich pattern:

```
CombatState (Rich Domain Object)
├── Initiative Rolling & Management
├── Turn Progression Logic
├── Event Emission
└── ToData() for Persistence

CombatStateData (Data Object)
├── All state for persistence
├── JSON serializable
└── LoadCombatStateFromContext()

Event System Integration
├── Combat lifecycle events
├── Initiative events
└── Turn/round events
```

## Core Components

### CombatState

Rich domain object that manages combat state with methods for:
- Adding/removing combatants
- Rolling initiative with multiple modes
- Resolving ties
- Starting combat and managing turns
- Converting to/from persistent data

### CombatStateData

Data structure containing all information needed to persist and reconstruct combat:
- Combat metadata (ID, name, status)
- Round and turn tracking
- Initiative order with full details
- Combatant data and statistics
- Combat settings

### Initiative System

Comprehensive initiative handling with:
- **Roll Mode**: Standard 1d20 + DEX modifier
- **Static Mode**: Use 10 + DEX modifier (no randomness)
- **Manual Mode**: DM sets initiative values directly

### Tie Breaking

Multiple tie resolution methods:
- **Dexterity**: Higher DEX score wins (D&D 5e default)
- **DM Decision**: Manual ordering by the DM
- **Re-roll**: Roll another d20 to break ties

## Usage Example

```go
// Create combat encounter
combat := combat.NewCombatState(combat.CombatStateConfig{
    ID:       "encounter-001",
    Name:     "Goblin Ambush",
    EventBus: eventBus,
    Settings: combat.CombatSettings{
        InitiativeRollMode: combat.InitiativeRollModeRoll,
        TieBreakingMode:   combat.TieBreakingModeDexterity,
    },
})

// Add combatants (must implement combat.Combatant interface)
for _, combatant := range combatants {
    err := combat.AddCombatant(combatant)
    // handle error
}

// Roll initiative
rollOutput, err := combat.RollInitiative(&combat.RollInitiativeInput{
    Combatants: combatants,
    RollMode:   combat.InitiativeRollModeRoll,
})

// Resolve any ties
if len(rollOutput.UnresolvedTies) > 0 {
    _, err = combat.ResolveTies(&combat.ResolveTiesInput{
        TiedGroups:        rollOutput.UnresolvedTies,
        InitiativeEntries: rollOutput.InitiativeEntries,
        TieBreakingMode:   combat.TieBreakingModeDexterity,
    })
}

// Start combat
err = combat.StartCombat()

// Progress through turns
err = combat.NextTurn()
```

## Persistence

The system supports full persistence using the GameContext pattern:

```go
// Save combat state
data := combat.ToData()
// ... save data to database/file

// Load combat state
gameCtx, err := game.NewContext(eventBus, data)
loadedCombat, err := combat.LoadCombatStateFromContext(ctx, gameCtx)
```

## Events

The system emits detailed events for all state changes:

### Combat Lifecycle
- `dnd5e.combat.started` - Combat begins
- `dnd5e.combat.ended` - Combat concludes
- `dnd5e.combat.paused/resumed` - Combat state changes

### Initiative Events
- `dnd5e.combat.initiative.rolled` - Individual initiative roll
- `dnd5e.combat.initiative.order_set` - Initiative order established

### Turn Events
- `dnd5e.combat.turn.started` - Combatant's turn begins
- `dnd5e.combat.turn.ended` - Combatant's turn ends
- `dnd5e.combat.round.started` - New round begins
- `dnd5e.combat.round.ended` - Round concludes

### Combatant Events
- `dnd5e.combat.combatant.added` - Combatant joins
- `dnd5e.combat.combatant.removed` - Combatant leaves
- `dnd5e.combat.combatant.updated` - Combatant state changes

## Interfaces

### Combatant Interface

Entities participating in combat must implement:

```go
type Combatant interface {
    core.Entity
    
    // Initiative calculation
    GetDexterityModifier() int
    GetDexterityScore() int
    
    // Combat stats
    GetArmorClass() int
    GetHitPoints() int
    GetMaxHitPoints() int
    
    // Status checks
    IsConscious() bool
    IsDefeated() bool
}
```

## Testing

The package includes comprehensive tests using the testify suite pattern:

- Initiative rolling with all modes
- Tie resolution with all methods
- Turn progression and round tracking
- Event emission verification
- Persistence round-trip testing
- Edge case handling

Run tests with:
```bash
go test -v
```

## Integration with RPG Toolkit

This implementation follows all toolkit patterns:

- **Rich Domain Objects**: CombatState has methods and behavior
- **Data Separation**: CombatStateData for persistence only
- **Event-Driven**: Full event bus integration
- **GameContext Pattern**: Standard loading mechanism
- **Input/Output Types**: All methods use structured parameters
- **Thread Safety**: Proper mutex usage throughout

## D&D 5e Compliance

The implementation follows official D&D 5e rules:

- Initiative: 1d20 + DEX modifier
- Turn order: Highest initiative first
- Tie breaking: DEX score comparison (PHB p. 189)
- Round structure: All combatants act once per round
- Combat phases: Roll initiative → Start combat → Turn progression

## Future Extensions

The architecture supports future enhancements:

- Action economy tracking (action, bonus action, reaction)
- Condition and effect management
- Damage and healing integration
- Advanced combat options (ready actions, delays)
- Integration with spatial positioning system