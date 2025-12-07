# Type Safety and Self-Documenting Code Examples

This document demonstrates the modernized, type-safe approach to the encounter system implementation, following Go best practices.

## Core Principles

1. **Use `any` instead of `interface{}`** - Modern Go (1.18+)
2. **Typed constants over magic strings** - Self-documenting and compile-time safe
3. **Type aliases for clarity** - Make intent clear
4. **Exhaustive constant sets** - Document all valid options

## Behavior System Types

### Memory Management
```go
// MemoryKey ensures type-safe access to behavior memory
type MemoryKey string

const (
    // Combat memory
    MemoryKeyLastAttacker      MemoryKey = "last_attacker"       // Who hit me last
    MemoryKeyDamageTaken       MemoryKey = "damage_taken"        // Total damage this combat
    MemoryKeyOriginalPosition  MemoryKey = "original_position"   // Where I started
    
    // Tactical memory
    MemoryKeyEnemyPositions    MemoryKey = "enemy_positions"     // Known enemy locations
    MemoryKeyAllyPositions     MemoryKey = "ally_positions"      // Ally locations
    MemoryKeyEscapeRoutes      MemoryKey = "escape_routes"       // Calculated flee paths
    
    // Behavior memory
    MemoryKeyTargetPriority    MemoryKey = "target_priority"     // Current target focus
    MemoryKeyFleeThreshold     MemoryKey = "flee_threshold"      // When to run
    MemoryKeyRageLevel         MemoryKey = "rage_level"          // Berserker rage amount
)

// Type-safe memory access
type TypedMemory struct {
    data map[MemoryKey]any
}

func (m *TypedMemory) GetLastAttacker() (EntityID, bool) {
    val, ok := m.data[MemoryKeyLastAttacker]
    if !ok {
        return "", false
    }
    return val.(EntityID), true
}

func (m *TypedMemory) SetLastAttacker(id EntityID) {
    m.data[MemoryKeyLastAttacker] = id
}
```

### Action Types
```go
// ActionCategory groups related actions
type ActionCategory string

const (
    ActionCategoryMovement  ActionCategory = "movement"
    ActionCategoryCombat    ActionCategory = "combat"
    ActionCategoryMagic     ActionCategory = "magic"
    ActionCategorySupport   ActionCategory = "support"
    ActionCategoryDefensive ActionCategory = "defensive"
)

// ActionType with detailed documentation
type ActionType string

const (
    // Movement actions
    ActionTypeMove         ActionType = "move"          // Standard movement
    ActionTypeDash         ActionType = "dash"          // Double movement
    ActionTypeDisengage    ActionType = "disengage"     // Move without opportunity attacks
    ActionTypeTeleport     ActionType = "teleport"      // Instant position change
    
    // Combat actions
    ActionTypeAttack       ActionType = "attack"        // Basic weapon attack
    ActionTypeMultiattack  ActionType = "multiattack"   // Multiple attacks
    ActionTypeGrapple      ActionType = "grapple"       // Attempt to grapple
    ActionTypeShove        ActionType = "shove"         // Push target
    
    // Magic actions
    ActionTypeCastSpell    ActionType = "cast_spell"    // Cast a spell
    ActionTypeConcentrate  ActionType = "concentrate"   // Maintain concentration
    ActionTypeCounterspell ActionType = "counterspell"  // Counter enemy magic
    
    // Support actions
    ActionTypeHeal         ActionType = "heal"          // Restore HP
    ActionTypeBuff         ActionType = "buff"          // Apply positive effect
    ActionTypeDebuff       ActionType = "debuff"        // Apply negative effect
    
    // Defensive actions
    ActionTypeDodge        ActionType = "dodge"         // Disadvantage on attacks
    ActionTypeDefend       ActionType = "defend"        // Increase AC
    ActionTypeHide         ActionType = "hide"          // Become hidden
)

// Validate action combinations
func (a ActionType) Category() ActionCategory {
    switch a {
    case ActionTypeMove, ActionTypeDash, ActionTypeDisengage, ActionTypeTeleport:
        return ActionCategoryMovement
    case ActionTypeAttack, ActionTypeMultiattack, ActionTypeGrapple, ActionTypeShove:
        return ActionCategoryCombat
    case ActionTypeCastSpell, ActionTypeConcentrate, ActionTypeCounterspell:
        return ActionCategoryMagic
    case ActionTypeHeal, ActionTypeBuff, ActionTypeDebuff:
        return ActionCategorySupport
    case ActionTypeDodge, ActionTypeDefend, ActionTypeHide:
        return ActionCategoryDefensive
    default:
        return ActionCategory("unknown")
    }
}
```

### Targeting System
```go
// TargetingMode defines how actions select targets
type TargetingMode string

const (
    TargetingModeSelf         TargetingMode = "self"          // Target self only
    TargetingModeSingle       TargetingMode = "single"        // One target
    TargetingModeMultiple     TargetingMode = "multiple"      // Multiple targets
    TargetingModeArea         TargetingMode = "area"          // Area of effect
    TargetingModeAll          TargetingMode = "all"           // All valid targets
)

// TargetFilter constrains valid targets
type TargetFilter string

const (
    TargetFilterEnemy         TargetFilter = "enemy"          // Hostile targets
    TargetFilterAlly          TargetFilter = "ally"           // Friendly targets
    TargetFilterSelf          TargetFilter = "self"           // Only self
    TargetFilterNonSelf       TargetFilter = "non_self"       // Anyone but self
    TargetFilterLiving        TargetFilter = "living"         // Not dead/unconscious
    TargetFilterUndead        TargetFilter = "undead"         // Undead only
    TargetFilterVisible       TargetFilter = "visible"        // Can see
    TargetFilterInRange       TargetFilter = "in_range"       // Within ability range
)

// Combine for complex targeting
type TargetingRequirements struct {
    Mode     TargetingMode
    Filters  []TargetFilter
    MinCount int
    MaxCount int
    Range    float64
}

// Example: Healing spell targets
var HealingTargets = TargetingRequirements{
    Mode:     TargetingModeSingle,
    Filters:  []TargetFilter{TargetFilterAlly, TargetFilterLiving, TargetFilterVisible},
    MinCount: 1,
    MaxCount: 1,
    Range:    30.0,
}
```

### Condition System
```go
// ConditionType for status effects
type ConditionType string

const (
    // Debilitating conditions
    ConditionTypeBlinded      ConditionType = "blinded"       // Can't see
    ConditionTypeCharmed      ConditionType = "charmed"       // Can't attack charmer
    ConditionTypeDeafened     ConditionType = "deafened"      // Can't hear
    ConditionTypeFrightened   ConditionType = "frightened"    // Disadvantage near source
    ConditionTypeGrappled     ConditionType = "grappled"      // Speed 0
    ConditionTypeIncapacitated ConditionType = "incapacitated" // Can't act
    ConditionTypeInvisible    ConditionType = "invisible"     // Can't be seen
    ConditionTypeParalyzed    ConditionType = "paralyzed"     // Can't move or act
    ConditionTypePetrified    ConditionType = "petrified"     // Turned to stone
    ConditionTypePoisoned     ConditionType = "poisoned"      // Disadvantage on attacks
    ConditionTypeProne        ConditionType = "prone"         // On the ground
    ConditionTypeRestrained   ConditionType = "restrained"    // Speed 0, disadvantage
    ConditionTypeStunned      ConditionType = "stunned"       // Incapacitated
    ConditionTypeUnconscious  ConditionType = "unconscious"   // Helpless
    
    // Beneficial conditions
    ConditionTypeBlessed      ConditionType = "blessed"       // Bonus to rolls
    ConditionTypeHasted       ConditionType = "hasted"        // Extra action
    ConditionTypeInspired     ConditionType = "inspired"      // Inspiration die
    ConditionTypeRaging       ConditionType = "raging"        // Damage resistance
    ConditionTypeShielded     ConditionType = "shielded"      // AC bonus
)

// Condition effects on behavior
func (c ConditionType) AffectsBehavior() bool {
    switch c {
    case ConditionTypeCharmed, ConditionTypeFrightened, ConditionTypeRaging:
        return true
    default:
        return false
    }
}
```

### Event Types
```go
// CombatEventType for encounter events
type CombatEventType string

const (
    // Turn events
    CombatEventTurnStart       CombatEventType = "turn_start"
    CombatEventTurnEnd         CombatEventType = "turn_end"
    CombatEventRoundStart      CombatEventType = "round_start"
    CombatEventRoundEnd        CombatEventType = "round_end"
    
    // Action events
    CombatEventActionDeclared  CombatEventType = "action_declared"
    CombatEventActionExecuted  CombatEventType = "action_executed"
    CombatEventActionFailed    CombatEventType = "action_failed"
    
    // Combat events
    CombatEventAttackRoll      CombatEventType = "attack_roll"
    CombatEventDamageDealt     CombatEventType = "damage_dealt"
    CombatEventHealingDone     CombatEventType = "healing_done"
    CombatEventConditionApplied CombatEventType = "condition_applied"
    CombatEventConditionRemoved CombatEventType = "condition_removed"
    
    // State changes
    CombatEventEntityDowned    CombatEventType = "entity_downed"
    CombatEventEntityKilled    CombatEventType = "entity_killed"
    CombatEventEntityFled      CombatEventType = "entity_fled"
    CombatEventCombatEnd       CombatEventType = "combat_end"
)

// Event severity for logging/filtering
type EventSeverity string

const (
    EventSeverityDebug    EventSeverity = "debug"     // AI decisions, pathfinding
    EventSeverityInfo     EventSeverity = "info"      // Normal combat flow
    EventSeverityImportant EventSeverity = "important" // Kills, critical hits
    EventSeverityCritical EventSeverity = "critical"  // Combat end, TPK
)
```

### Spatial Types
```go
// DistanceUnit for measurements
type DistanceUnit string

const (
    DistanceUnitFeet   DistanceUnit = "feet"
    DistanceUnitMeters DistanceUnit = "meters"
    DistanceUnitSquares DistanceUnit = "squares" // Grid squares
)

// MovementType affects pathfinding
type MovementType string

const (
    MovementTypeWalk     MovementType = "walk"     // Normal ground movement
    MovementTypeFly      MovementType = "fly"      // Aerial movement
    MovementTypeSwim     MovementType = "swim"     // Water movement
    MovementTypeBurrow   MovementType = "burrow"   // Underground movement
    MovementTypeClimb    MovementType = "climb"    // Vertical movement
    MovementTypeTeleport MovementType = "teleport" // Instant travel
)

// TerrainType affects movement cost
type TerrainType string

const (
    TerrainTypeNormal       TerrainType = "normal"        // Standard movement
    TerrainTypeDifficult    TerrainType = "difficult"     // Half speed
    TerrainTypeImpassable   TerrainType = "impassable"    // Can't enter
    TerrainTypeHazardous    TerrainType = "hazardous"     // Damages on entry
    TerrainTypeWater        TerrainType = "water"         // Requires swim
    TerrainTypeLava         TerrainType = "lava"          // Extreme hazard
)
```

## Usage Examples

### Type-Safe Behavior Implementation
```go
type AggressiveBehavior struct {
    targetPriority []TargetPriority
    actionWeights  map[ActionType]float64
}

func (b *AggressiveBehavior) Execute(ctx BehaviorContext) (Action, error) {
    // Type-safe memory access
    memory := ctx.GetTypedMemory()
    if lastAttacker, ok := memory.GetLastAttacker(); ok {
        // Focus on whoever hurt us
        return b.createAttackAction(lastAttacker), nil
    }
    
    // Type-safe perception check
    perception := ctx.GetPerception()
    if perception.Type != PerceptionTypeVisual {
        // Can't see, use different strategy
        return b.blindAttackStrategy(ctx)
    }
    
    // Find target using typed priorities
    target := b.selectTarget(perception.VisibleEntities, b.targetPriority)
    if target == nil {
        return Action{Type: ActionTypeMove}, nil
    }
    
    // Select action based on weights
    action := b.selectWeightedAction(b.actionWeights)
    return b.createAction(action, target), nil
}

func (b *AggressiveBehavior) selectWeightedAction(weights map[ActionType]float64) ActionType {
    // Implementation using selectables with type safety
    table := selectables.NewTable[ActionType]()
    for action, weight := range weights {
        table.Add(action, weight)
    }
    return table.Select()
}
```

### Type-Safe Event Publishing
```go
type CombatEventPublisher struct {
    bus events.Bus
}

func (p *CombatEventPublisher) PublishDamageEvent(
    attacker EntityID,
    target EntityID,
    amount int,
    damageType DamageType,
) {
    event := DamageDealtEvent{
        Type:       CombatEventDamageDealt,
        Severity:   EventSeverityInfo,
        Timestamp:  time.Now(),
        AttackerID: attacker,
        TargetID:   target,
        Amount:     amount,
        DamageType: damageType,
    }
    
    // Special handling for kills
    if p.isKillingBlow(target, amount) {
        event.Severity = EventSeverityImportant
    }
    
    p.bus.Publish(event)
}
```

## Benefits

1. **Compile-time Safety**: Typos in strings become compile errors
2. **IDE Support**: Auto-completion for all constants
3. **Self-Documenting**: Constants include documentation
4. **Refactoring**: Change a constant name, compiler finds all usages
5. **Type Validation**: Can't pass invalid values to functions
6. **Maintainability**: New developers understand valid options immediately

This approach makes the codebase more robust, easier to understand, and significantly reduces runtime errors from typos or invalid values.