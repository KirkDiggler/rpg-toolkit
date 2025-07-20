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
// MemoryKey represents typed keys for behavior memory storage
type MemoryKey string

const (
    MemoryKeyLastAttacker   MemoryKey = "last_attacker"
    MemoryKeyLastPosition   MemoryKey = "last_position"
    MemoryKeyTargetPriority MemoryKey = "target_priority"
    MemoryKeyFleeThreshold  MemoryKey = "flee_threshold"
    MemoryKeyAllyPositions  MemoryKey = "ally_positions"
)

type BehaviorContext interface {
    Entity() core.Entity
    GetPerception() PerceptionData
    GetSpatialInfo() SpatialInfo
    GetMemory(key MemoryKey) any
    SetMemory(key MemoryKey, value any)
    PublishDecision(decision Decision)
}
```

#### 2. Behavior Interface
```go
// BehaviorPriority defines the execution order of behaviors
type BehaviorPriority int

const (
    BehaviorPriorityEmergency BehaviorPriority = 1000 // Flee, death saves
    BehaviorPriorityDefensive BehaviorPriority = 800  // Healing, defensive stance
    BehaviorPriorityCombat    BehaviorPriority = 600  // Attack, spell casting
    BehaviorPrioritySupport   BehaviorPriority = 400  // Buff allies, debuff enemies
    BehaviorPriorityDefault   BehaviorPriority = 200  // Standard movement, patrol
    BehaviorPriorityIdle      BehaviorPriority = 0    // No immediate action needed
)

type Behavior interface {
    // Execute returns the next action to take
    Execute(ctx BehaviorContext) (Action, error)
    
    // CanExecute checks if this behavior applies
    CanExecute(ctx BehaviorContext) bool
    
    // Priority for behavior selection
    Priority() BehaviorPriority
}
```

#### 3. Multiple Paradigm Support

**State Machine**:
```go
// StateID represents a unique state identifier
type StateID string

const (
    StateIDIdle       StateID = "idle"
    StateIDPatrol     StateID = "patrol"
    StateIDAlert      StateID = "alert"
    StateIDCombat     StateID = "combat"
    StateIDFleeing    StateID = "fleeing"
    StateIDSupporting StateID = "supporting"
    StateIDDead       StateID = "dead"
)

type StateMachineBehavior struct {
    states  map[StateID]State
    current StateID
}

type State interface {
    ID() StateID
    Enter(ctx BehaviorContext) error
    Execute(ctx BehaviorContext) (nextState StateID, action Action, err error)
    Exit(ctx BehaviorContext) error
}
```

**Behavior Tree**:
```go
// NodeStatus represents the execution status of a behavior tree node
type NodeStatus string

const (
    NodeStatusRunning NodeStatus = "running" // Still executing
    NodeStatusSuccess NodeStatus = "success" // Completed successfully
    NodeStatusFailure NodeStatus = "failure" // Failed to complete
)

type BehaviorNode interface {
    Execute(ctx BehaviorContext) NodeResult
}

type NodeResult struct {
    Status NodeStatus
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
// BehaviorType identifies the type of behavior making decisions
type BehaviorType string

const (
    BehaviorTypeAggressive BehaviorType = "aggressive"
    BehaviorTypeTactical   BehaviorType = "tactical"
    BehaviorTypeFrightened BehaviorType = "frightened"
    BehaviorTypeSupport    BehaviorType = "support"
    BehaviorTypeBerserker  BehaviorType = "berserker"
    BehaviorTypeDefensive  BehaviorType = "defensive"
)

// ActionType represents the type of action being taken
type ActionType string

const (
    ActionTypeMove           ActionType = "move"
    ActionTypeAttack         ActionType = "attack"
    ActionTypeCast           ActionType = "cast"
    ActionTypeDefend         ActionType = "defend"
    ActionTypeFlee           ActionType = "flee"
    ActionTypeHeal           ActionType = "heal"
    ActionTypeHide           ActionType = "hide"
    ActionTypeInteract       ActionType = "interact"
    ActionTypeWait           ActionType = "wait"
)

// Decision made
behaviorEvent.DecisionMade{
    EntityID:     "goblin-1",
    Behavior:     BehaviorTypeAggressive,
    ChosenAction: ActionTypeAttack,
    Reasoning:    "nearest enemy in range",
}

// State changed (for state machines)
behaviorEvent.StateChanged{
    EntityID:  "wizard-1",
    FromState: StateIDAlert,
    ToState:   StateIDCombat,
    Trigger:   "spotted enemy",
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