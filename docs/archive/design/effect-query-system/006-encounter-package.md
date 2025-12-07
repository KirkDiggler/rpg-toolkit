# Encounter Package Design

**Date:** 2024-12-01
**Status:** Early Exploration
**Builds on:** 004-hybrid-design.md

## The Insight

We don't need an SDK - we need an **encounter** package in the toolkit. This is a tool that:

1. Manages a turn-based encounter lifecycle
2. Loads all data needed for a turn
3. Hooks everything up to the bus
4. Executes the turn
5. Tracks changes for persistence
6. Returns results

## Where It Lives

```
rpg-toolkit/
├── events/           # Event bus infrastructure
├── tools/
│   ├── spatial/      # Room, positions, movement
│   ├── spawn/        # Entity spawning
│   └── encounter/    # NEW: Turn-based encounter management
└── rulebooks/
    └── dnd5e/        # Uses encounter package
```

The encounter package sits **below** rulebooks, **alongside** spatial. It's infrastructure that rulebooks consume.

## The Turn Lifecycle

```
┌─────────────────────────────────────────────────────────────┐
│                    Encounter.TakeTurn()                     │
├─────────────────────────────────────────────────────────────┤
│ 1. LOAD                                                     │
│    - Load character state from storage                      │
│    - Load active conditions/effects                         │
│    - Load room/spatial data                                 │
│    - Load combat state (initiative, round)                  │
├─────────────────────────────────────────────────────────────┤
│ 2. SETUP                                                    │
│    - Create event bus for this turn                         │
│    - Apply all conditions to bus (subscribe to topics)      │
│    - Set up context with encounter data                     │
│    - Context available to all effects                       │
├─────────────────────────────────────────────────────────────┤
│ 3. EXECUTE                                                  │
│    - Process player action(s)                               │
│    - Effects fire as events flow                            │
│    - Query topics available for lookups                     │
│    - Mutations tracked                                      │
├─────────────────────────────────────────────────────────────┤
│ 4. COLLECT                                                  │
│    - Gather all changes (HP, conditions, positions)         │
│    - Serialize modified state                               │
│    - Build response with results                            │
├─────────────────────────────────────────────────────────────┤
│ 5. PERSIST                                                  │
│    - Save changes back to storage                           │
│    - Return response to caller                              │
└─────────────────────────────────────────────────────────────┘
```

## Core Types

```go
// tools/encounter/encounter.go

// Encounter manages a turn-based game encounter.
// It handles the lifecycle of loading state, executing turns,
// and persisting changes.
type Encounter struct {
    id           string
    bus          events.EventBus
    participants map[string]*Participant
    room         *spatial.Room  // Optional, if spatial matters
    round        int
    turnOrder    []string
    currentTurn  int

    // Change tracking
    changes      *ChangeSet
}

// Participant represents an entity in the encounter
type Participant struct {
    ID         string
    Entity     core.Entity
    Team       Team
    Position   *spatial.Position  // nil if no spatial
    Conditions []effect.BusEffect // Active conditions
    // Character data loaded from storage
}

// TurnInput is what the player/AI wants to do
type TurnInput struct {
    ActorID string
    Actions []Action  // Move, Attack, Cast, etc.
}

// TurnResult is what happened
type TurnResult struct {
    ActorID     string
    Round       int
    Actions     []ActionResult
    Changes     *ChangeSet  // What changed (for persistence)
    NextTurnID  string      // Who goes next
}
```

## The Context Setup

```go
// During SETUP phase, encounter builds context

func (e *Encounter) setupContext(ctx context.Context) context.Context {
    ec := &EncounterContext{
        EncounterID:  e.id,
        Round:        e.round,
        CurrentTurn:  e.turnOrder[e.currentTurn],
        Participants: e.buildParticipantList(),
        Positions:    e.buildPositionMap(),
    }
    return WithEncounterContext(ctx, ec)
}

// Effects access it naturally
func (s *SneakAttackCondition) onDamageChain(ctx context.Context, ...) {
    enc := GetEncounterContext(ctx)
    if enc != nil && enc.HasAllyAdjacentTo(s.CharacterID, targetID) {
        // Add sneak attack damage
    }
}
```

## Change Tracking

```go
// ChangeSet tracks mutations during a turn
type ChangeSet struct {
    HPChanges        map[string]int           // entity -> new HP
    ConditionsAdded  map[string][]Condition   // entity -> new conditions
    ConditionsRemoved map[string][]string     // entity -> removed condition IDs
    PositionChanges  map[string]Position      // entity -> new position
    ResourceChanges  map[string]ResourceDelta // entity -> resource changes
}

// During execution, encounter tracks changes via event subscriptions
func (e *Encounter) setupChangeTracking(ctx context.Context) {
    // Subscribe to damage events
    damages := dnd5eEvents.DamageReceivedTopic.On(e.bus)
    damages.Subscribe(ctx, func(ctx context.Context, event DamageReceivedEvent) error {
        e.changes.TrackDamage(event.TargetID, event.Amount)
        return nil
    })

    // Subscribe to condition events
    conditions := dnd5eEvents.ConditionAppliedTopic.On(e.bus)
    conditions.Subscribe(ctx, func(ctx context.Context, event ConditionAppliedEvent) error {
        e.changes.TrackConditionAdded(event.Target.GetID(), event.Condition)
        return nil
    })

    // etc.
}
```

## Usage from Game Server

```go
// In rpg-api, handling a turn request

func (s *CombatService) TakeTurn(ctx context.Context, req *TakeTurnRequest) (*TakeTurnResponse, error) {
    // Load encounter from storage
    enc, err := s.loadEncounter(ctx, req.EncounterID)
    if err != nil {
        return nil, err
    }

    // Build input from request
    input := &encounter.TurnInput{
        ActorID: req.CharacterID,
        Actions: convertActions(req.Actions),
    }

    // Execute turn - encounter handles everything
    result, err := enc.TakeTurn(ctx, input)
    if err != nil {
        return nil, err
    }

    // Persist changes
    if err := s.saveChanges(ctx, result.Changes); err != nil {
        return nil, err
    }

    // Return response
    return convertToResponse(result), nil
}
```

## Why This Is Powerful

1. **Single responsibility**: Encounter manages the turn lifecycle
2. **Reusable**: Any turn-based game can use this pattern
3. **Testable**: Encounter is pure - give it data, get results
4. **Clean boundaries**:
   - Storage concerns stay in game server
   - Rules concerns stay in rulebook
   - Lifecycle concerns stay in encounter
5. **Context is natural**: Encounter has all the data, sets up context

## Integration with Query System

The encounter package would work seamlessly with query topics:

```go
// Character wants to know their AC during encounter
func (e *Encounter) GetParticipantAC(ctx context.Context, participantID string) (*ACResult, error) {
    ctx = e.setupContext(ctx)  // Ensure context is set

    p := e.participants[participantID]
    input := buildACInput(p)

    return queries.QueryAC(ctx, e.bus, participantID, input)
}
```

## Open Questions (For Future)

1. How does encounter get instantiated? (Factory? Loader?)
2. How to handle reactions (opportunity attacks happen on other's turns)?
3. How to handle simultaneous actions (multiple creatures same initiative)?
4. Should encounter own the bus or receive it?
5. How to handle encounters without spatial (theater of mind)?

## Relationship to Other Packages

```
events/          - Bus infrastructure (Encounter uses)
tools/spatial/   - Room/position management (Encounter integrates)
tools/spawn/     - Entity spawning (Encounter could use for summons)
tools/encounter/ - Turn lifecycle management (NEW)
rulebooks/dnd5e/ - Game rules (Uses encounter for combat)
```

## This Is Future Work

We're documenting this now to:
1. Validate the query/context design makes sense with encounter
2. Ensure we don't paint ourselves into a corner
3. Have a north star for where we're heading

**Next immediate steps remain:**
1. Clean up unified grant system (current branch)
2. Implement query topics for AC, saves
3. Then consider encounter package when we need context
