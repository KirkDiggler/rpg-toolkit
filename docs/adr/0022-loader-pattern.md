# ADR-0022: Loader Pattern for Domain Object Creation

## Status
Proposed

## Context

We've been mixing responsibilities:
- Repositories were trying to create domain objects
- Orchestrators were doing both loading and business logic
- No clear separation between data retrieval and domain object creation

Key insight: Orchestrators should handle complete workflows, not be reusable pieces. The reusable pieces are the loaders.

## Decision

**Introduce a Loader layer between Repository and Orchestrator.**

```
Repository (data) → Loader (domain objects) → Orchestrator (complete workflow)
```

### Repository
- Pure data storage/retrieval
- No domain logic
- Returns data structs

```go
type Repository interface {
    Get(ctx context.Context, input *GetInput) (*GetOutput, error)
    Save(ctx context.Context, input *SaveInput) (*SaveOutput, error)
}
```

### Loader
- Converts data to domain objects
- Wires up event bus connections
- Activates features/effects
- **Reusable across orchestrators**

```go
type CharacterLoader interface {
    Load(ctx context.Context, input *LoadInput) (*LoadOutput, error)
}

type LoadInput struct {
    CharacterID string
    EventBus    events.EventBus
}

type LoadOutput struct {
    Character *dnd5e.Character  // Fully activated domain object
}
```

### Orchestrator
- Orchestrates complete workflows
- Uses multiple loaders
- Handles business logic
- **Not meant to be reusable**

```go
type PlayerTurnOrchestrator struct {
    charLoader    *loaders.CharacterLoader
    monsterLoader *loaders.MonsterLoader
    roomLoader    *loaders.RoomLoader
}

func (o *PlayerTurnOrchestrator) ExecuteTurn(ctx context.Context, input *ExecuteTurnInput) (*ExecuteTurnOutput, error) {
    // Load everything needed
    char := o.charLoader.Load(...)
    monsters := o.monsterLoader.Load(...)
    room := o.roomLoader.Load(...)
    
    // Orchestrate complete turn
    // ...
}
```

## Consequences

### Positive
- Clear separation of concerns
- Loaders are reusable components
- Orchestrators handle complete workflows
- Repository stays simple (just data)
- Easier to test each layer

### Negative
- Additional layer of abstraction
- More interfaces to maintain

### Neutral
- Loaders become the reusable building blocks
- Orchestrators become workflow controllers

## Examples

### Good Orchestrator (Complete Workflow)
- `PlayerTurnOrchestrator` - Handles entire player turn
- `CombatEncounterOrchestrator` - Manages full combat
- `CharacterCreationOrchestrator` - Complete character creation flow

### Bad Orchestrator (Too Small)
- `GetCharacterOrchestrator` - Just loads, should be a loader
- `ActivateFeatureOrchestrator` - Too granular

## Migration Path

1. Create `internal/loaders/` directory
2. Move loading logic from orchestrators to loaders
3. Simplify repositories to just data operations
4. Refactor orchestrators to use loaders