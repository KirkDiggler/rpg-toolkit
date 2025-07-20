# ADR-0016: Behavior System Architecture

## Status
Proposed

## Context
The toolkit needs infrastructure for entity behaviors, particularly for AI-controlled entities in combat encounters. This system must:
- Support multiple behavior paradigms (state machines, behavior trees, utility AI)
- Integrate with existing spatial and event systems
- Remain game-agnostic (no D&D-specific logic)
- Enable observable, testable AI decisions
- Scale from simple to complex behaviors

## Decision

### Core Architecture

We will implement a pluggable behavior system with these components:

#### 1. Behavior Context
```go
type BehaviorContext interface {
    Entity() core.Entity
    GetPerception() PerceptionData
    GetSpatialInfo() SpatialInfo
    GetMemory(key string) interface{}
    SetMemory(key string, value interface{})
    PublishDecision(decision Decision)
}
```

#### 2. Behavior Interface
```go
type Behavior interface {
    // Execute returns the next action to take
    Execute(ctx BehaviorContext) (Action, error)
    
    // CanExecute checks if this behavior applies
    CanExecute(ctx BehaviorContext) bool
    
    // Priority for behavior selection
    Priority() int
}
```

#### 3. Multiple Paradigm Support

**State Machine**:
```go
type StateMachineBehavior struct {
    states map[string]State
    current string
}

type State interface {
    Enter(ctx BehaviorContext) error
    Execute(ctx BehaviorContext) (nextState string, action Action, err error)
    Exit(ctx BehaviorContext) error
}
```

**Behavior Tree**:
```go
type BehaviorNode interface {
    Execute(ctx BehaviorContext) NodeResult
}

type NodeResult struct {
    Status Status // Running, Success, Failure
    Action *Action
}
```

**Utility AI**:
```go
type UtilityBehavior struct {
    evaluators []UtilityEvaluator
}

type UtilityEvaluator interface {
    Score(ctx BehaviorContext) float64
    GetAction() Action
}
```

#### 4. Integration Points

**Perception System**:
```go
type PerceptionSystem interface {
    GetVisibleEntities(observer core.Entity, room *spatial.Room) []core.Entity
    GetAudibleEvents(observer core.Entity, timeWindow time.Duration) []Event
    CanSee(observer, target core.Entity, room *spatial.Room) bool
}
```

**Action Queue**:
```go
type ActionQueue interface {
    QueueAction(entity core.Entity, action Action) error
    ProcessNext() (*ExecutedAction, error)
    GetPendingActions(entity core.Entity) []Action
}
```

### Event Integration

All behavior decisions publish events:
```go
// Decision made
behaviorEvent.DecisionMade{
    EntityID: "goblin-1",
    Behavior: "aggressive",
    ChosenAction: "attack",
    Reasoning: "nearest enemy in range",
}

// State changed (for state machines)
behaviorEvent.StateChanged{
    EntityID: "wizard-1",
    FromState: "exploring",
    ToState: "combat",
    Trigger: "spotted enemy",
}
```

### No Implementation, Only Infrastructure

The toolkit provides:
- Interfaces and base types
- Event definitions
- Integration with spatial/perception
- Helper utilities (pathfinding, line-of-sight)

Games provide:
- Concrete behaviors
- Action definitions
- Decision logic
- AI personalities

## Consequences

### Positive
- **Flexible**: Supports simple to complex AI without forcing a paradigm
- **Observable**: Event-driven makes AI decisions visible for debugging
- **Testable**: Clean interfaces enable unit testing of behaviors
- **Game-agnostic**: No RPG-specific logic in the toolkit
- **Extensible**: New behavior types can be added without breaking existing ones

### Negative
- **Complexity**: Multiple paradigms mean more to learn/maintain
- **No defaults**: Games must implement all behaviors from scratch
- **Integration burden**: Games must wire perception, spatial, and behaviors together

### Neutral
- **Performance**: Behavior execution should be fast, but complex perception might be costly
- **Memory**: Context and memory storage per entity could add up

## Implementation Notes

1. Start with basic interfaces and state machine support
2. Add behavior tree support based on demand
3. Perception system can begin simple (visible = in same room)
4. Pathfinding can use A* on spatial grid
5. Consider behavior composition (combining multiple behaviors)

## References
- Journey Document 017: Encounter System Design
- ADR-0009: Multi-Room Orchestration (spatial foundation)
- ADR-0012: Selectables Tool Architecture (for weighted decisions)
- Event system documentation