# Encounter System Implementation Plan

## Overview

This document outlines the concrete implementation plan for the encounter system across all RPG ecosystem projects. It builds on the architectural decisions in ADR-0016 and the journey documented in 017.

## Phase 1: Foundation (Toolkit Infrastructure)

### 1.1 Spawn Engine Implementation

**Prerequisites**: 
- Spatial system (✓ Complete)
- Selectables system (✓ Complete)

**Deliverables**:
```go
// SpawnDensity defines how many entities to spawn
type SpawnDensity string

const (
    SpawnDensityLight    SpawnDensity = "light"    // 1-2 entities per 100 sq ft
    SpawnDensityModerate SpawnDensity = "moderate" // 3-4 entities per 100 sq ft
    SpawnDensityHeavy    SpawnDensity = "heavy"    // 5-6 entities per 100 sq ft
    SpawnDensityPacked   SpawnDensity = "packed"   // 7+ entities per 100 sq ft
)

// EntityPoolType categorizes groups of entities
type EntityPoolType string

const (
    EntityPoolTypeMonsters    EntityPoolType = "monsters"
    EntityPoolTypeTreasure    EntityPoolType = "treasure"
    EntityPoolTypeHazards     EntityPoolType = "hazards"
    EntityPoolTypeNPCs        EntityPoolType = "npcs"
    EntityPoolTypeInteractive EntityPoolType = "interactive"
)

// Core spawn interfaces
type SpawnEngine interface {
    PopulateRoom(roomID string, config SpawnConfig) (SpawnResult, error)
    PlaceEntity(room *spatial.Room, entity core.Entity, constraints PlacementConstraints) error
}

type SpawnConfig struct {
    EntityPools     map[EntityPoolType][]core.Entity  // Pre-created entities by category
    SelectionTables map[EntityPoolType]*selectables.Table // What to spawn
    Density         SpawnDensity                      // How many to spawn
    Constraints     []PlacementConstraint             // Where to place
}
```

**Implementation Steps**:
1. Create spawn point system in spatial module
2. Implement capacity calculation (space per entity)
3. Build constraint relaxation for guaranteed placement
4. Integrate with Selectables for random selection
5. Add spawn events for observability

### 1.2 Basic Behavior Infrastructure

**Deliverables**:
```go
// PerceptionType categorizes what was perceived
type PerceptionType string

const (
    PerceptionTypeVisual   PerceptionType = "visual"   // Seen entities
    PerceptionTypeAuditory PerceptionType = "auditory" // Heard sounds
    PerceptionTypeTactile  PerceptionType = "tactile"  // Felt vibrations
    PerceptionTypeMagical  PerceptionType = "magical"  // Detected via magic
)

// Perception system
type PerceptionData struct {
    VisibleEntities []PerceivedEntity
    AudibleEvents   []PerceivedEvent
    RoomLayout      *spatial.RoomInfo
    Type            PerceptionType
}

// PathfindingAlgorithm specifies which algorithm to use
type PathfindingAlgorithm string

const (
    PathfindingAlgorithmAStar     PathfindingAlgorithm = "astar"     // A* for optimal paths
    PathfindingAlgorithmDijkstra  PathfindingAlgorithm = "dijkstra"  // For multiple targets
    PathfindingAlgorithmGreedy    PathfindingAlgorithm = "greedy"    // Fast but suboptimal
    PathfindingAlgorithmStraight  PathfindingAlgorithm = "straight"  // Direct line if possible
)

// Basic pathfinding
type Pathfinder interface {
    FindPath(from, to spatial.Position, room *spatial.Room, algorithm PathfindingAlgorithm) ([]spatial.Position, error)
    GetReachablePositions(from spatial.Position, maxDistance float64) []spatial.Position
}

// ActionPriority defines execution order in the queue
type ActionPriority int

const (
    ActionPriorityImmediate ActionPriority = 1000 // Reactions, interrupts
    ActionPriorityHigh      ActionPriority = 800  // Bonus actions
    ActionPriorityNormal    ActionPriority = 500  // Standard actions
    ActionPriorityLow       ActionPriority = 200  // Delayed effects
)

// Action queue for turn-based combat
type ActionQueue interface {
    QueueAction(entity core.Entity, action Action, priority ActionPriority) error
    GetNext() (core.Entity, Action, error)
    Clear(entity core.Entity) error
}
```

## Phase 2: Combat Orchestration (API Layer)

### 2.1 Encounter Service

**gRPC Service Definition**:
```protobuf
service EncounterService {
    rpc CreateEncounter(CreateEncounterRequest) returns (Encounter);
    rpc JoinEncounter(JoinEncounterRequest) returns (EncounterState);
    rpc ProcessTurn(ProcessTurnRequest) returns (TurnResult);
    rpc GetState(GetStateRequest) returns (EncounterState);
    rpc StreamEvents(StreamEventsRequest) returns (stream EncounterEvent);
}

message Encounter {
    string id = 1;
    string room_id = 2;
    repeated Participant participants = 3;
    CombatState state = 4;
}
```

### 2.2 Turn Management

**Core Logic**:
```go
type TurnManager struct {
    participants []TurnParticipant
    currentTurn  int
    round        int
}

func (tm *TurnManager) ProcessTurn(action Action) (TurnResult, error) {
    // 1. Validate action for current participant
    // 2. Execute action (movement, attack, etc)
    // 3. Resolve effects (damage, conditions)
    // 4. Check for state changes (death, victory)
    // 5. Advance to next turn
    // 6. Trigger AI for NPCs if their turn
}
```

### 2.3 Monster Templates

**Template Structure**:
```go
// TargetPriority defines how AI selects targets
type TargetPriority string

const (
    TargetPriorityNearest        TargetPriority = "nearest"         // Closest enemy
    TargetPriorityMostDamaged    TargetPriority = "most_damaged"    // Lowest HP
    TargetPriorityLeastDamaged   TargetPriority = "least_damaged"   // Highest HP
    TargetPriorityHighestThreat  TargetPriority = "highest_threat"  // Most damage dealt
    TargetPriorityWeakest        TargetPriority = "weakest"         // Lowest defense
    TargetPriorityIsolated       TargetPriority = "isolated"        // Away from allies
    TargetPriorityRandom         TargetPriority = "random"          // Random selection
)

// RangePreference defines preferred combat distance
type RangePreference string

const (
    RangePreferenceMelee   RangePreference = "melee"   // Get close
    RangePreferenceRanged  RangePreference = "ranged"  // Keep distance
    RangePreferenceOptimal RangePreference = "optimal" // Best for abilities
)

// Template structure in Go (instead of YAML)
type MonsterTemplate struct {
    Name        string
    Description string
    TargetSelection struct {
        Priority []TargetPriority
        RangePref RangePreference
    }
    ActionWeights map[ActionType]float64
    Conditions    []BehaviorCondition
}

// Example: Aggressive template
var AggressiveTemplate = MonsterTemplate{
    Name:        "Aggressive",
    Description: "Charges nearest enemy, ignores safety",
    TargetSelection: struct {
        Priority []TargetPriority
        RangePref RangePreference
    }{
        Priority:  []TargetPriority{TargetPriorityNearest, TargetPriorityMostDamaged, TargetPriorityRandom},
        RangePref: RangePreferenceMelee,
    },
    ActionWeights: map[ActionType]float64{
        ActionTypeAttack: 0.8,
        ActionTypeMove:   0.2,
        ActionTypeCast:   0.0,
    },
}
```

## Phase 3: Visualization (Web App)

### 3.1 Spatial Rendering

**Components**:
```typescript
// Room renderer
interface RoomRenderer {
  renderRoom(room: RoomData): void
  renderGrid(gridType: GridType): void
  renderEntities(entities: Entity[]): void
  renderConnections(connections: Connection[]): void
}

// Entity tokens
interface EntityToken {
  position: Position
  sprite: Sprite
  healthBar: HealthDisplay
  statusEffects: StatusIcon[]
}
```

### 3.2 Animation System

**Key Animations**:
1. **Movement**: Smooth interpolation between grid positions
2. **Attacks**: Weapon swings, spell projectiles
3. **Damage**: Hit reactions, floating damage numbers
4. **Status**: Buff/debuff visual effects
5. **Death**: Fade out or death animation

**Implementation**:
```typescript
class CombatAnimationManager {
  queueMovement(entity: Entity, path: Position[]): Animation
  queueAttack(attacker: Entity, target: Entity, attackType: string): Animation
  queueSpellEffect(spell: Spell, targets: Entity[]): Animation
  playQueue(): Promise<void>
}
```

### 3.3 UI Components

**Essential UI**:
- Initiative tracker (turn order display)
- Action bar (available actions)
- Combat log (scrolling text of events)
- Entity inspector (click for details)
- Room minimap (for large encounters)

## Phase 4: AI Implementation Examples

### 4.1 State Machine Example (Goblin)

```go
type GoblinStates struct {
    Idle    *IdleState
    Alert   *AlertState
    Combat  *CombatState
    Fleeing *FleeingState
}

func (s *CombatState) Execute(ctx BehaviorContext) (StateID, Action, error) {
    perception := ctx.GetPerception()
    
    // Find nearest enemy
    nearest := findNearestEnemy(perception.VisibleEntities)
    if nearest == nil {
        return StateIDAlert, NoAction, nil
    }
    
    // Check if in melee range
    if distance(ctx.Entity().Position, nearest.Position) <= 5 {
        return "Combat", AttackAction{Target: nearest.ID}, nil
    }
    
    // Move closer
    path := ctx.GetPathfinder().FindPath(ctx.Entity().Position, nearest.Position)
    return "Combat", MoveAction{Path: path[0:1]}, nil
}
```

### 4.2 Behavior Tree Example (Wizard)

```go
root := Sequence{
    Children: []Node{
        // Check if concentrating on spell
        Condition{Check: IsConcentrating},
        Selector{
            Children: []Node{
                // Maintain concentration if valuable
                Sequence{
                    Children: []Node{
                        Condition{Check: ConcentrationWorthKeeping},
                        Action{Do: MaintainPosition},
                    },
                },
                // Break concentration for better option
                Action{Do: DropConcentration},
            },
        },
        // Cast best spell
        Selector{
            Children: []Node{
                TryCastFireball{},
                TryCastMagicMissile{},
                MoveToSafety{},
            },
        },
    },
}
```

## Performance Considerations

### Spatial Queries
- Use spatial hashing for O(1) entity lookups
- Cache line-of-sight calculations per turn
- Batch perception updates

### AI Decisions  
- Limit perception range to reduce entities considered
- Cache pathfinding results within same turn
- Use simple heuristics before complex calculations

### Animation
- Pool animation objects
- Use CSS transforms for movement
- Batch WebGL draw calls

## Testing Strategy

### Unit Tests
- Behavior state transitions
- Pathfinding edge cases
- Turn order management
- Action validation

### Integration Tests
- Full encounter flow
- Multi-room combat
- AI decision making
- State synchronization

### Load Tests
- 50+ entities in combat
- Complex behavior trees
- Pathfinding performance
- Animation frame rate

## Open Questions Resolved

1. **Pathfinding location**: In toolkit as infrastructure
2. **Behavior format**: YAML/JSON templates in API layer
3. **Animation timing**: Server sends events, client interpolates
4. **AI visibility**: Optional debug mode shows decision process
5. **Turn resolution**: Strict initiative order, simultaneous resolution per initiative count

## Success Metrics

- Phase 1: Spawn 20 goblins in a room programmatically
- Phase 2: Run 10-round combat with 6 participants  
- Phase 3: Smooth 60fps animation with 20 entities
- Phase 4: Monsters use terrain and focus fire intelligently

## Next Steps

1. Create GitHub issues for Phase 1 tasks
2. Set up behavior system test harness
3. Design encounter state schema
4. Prototype room renderer in React