# ADR-0024: Modification Pipeline Pattern for Effects

## Status
Proposed

## Context

While implementing the rage feature (PR #221), we discovered fundamental issues with how effects modify game values through events:

1. **Undefined Order**: Multiple effects subscribing to the same event with same priority leads to non-deterministic modification order
2. **Lost Context**: Flat key-value event data loses track of what modified what and why
3. **No Selective Loading**: Can't query for specific types of modifiers (e.g., "just attack modifiers")
4. **Difficult Testing**: Need full event bus to test simple modifications
5. **Debugging Challenges**: Can't easily see what modifications were applied in what order

Initially, we considered replacing the event system with a pull-based query system, but this would sacrifice the valuable decoupling that events provide (traps reacting to movement, curses responding to specific interactions, etc.).

## Decision

We will implement a **Modification Pipeline** pattern that works alongside our event system:

1. **Keep events for reactive behavior** - Maintain decoupling for triggers and reactions
2. **Add pipelines for calculations** - Use structured pipelines for predictable value modifications
3. **Rulebooks define stage order** - Each rulebook specifies the order of modification stages
4. **Effects declare their stage** - Effects specify which pipeline stage they belong to
5. **Track all modifications** - Maintain a record of what modified what and why

### Pipeline Structure

```go
// Core pipeline types
type Pipeline struct {
    Stages []Stage
}

type Stage string

type Modification struct {
    Stage      Stage
    Type       ModifierType
    Source     string        // What created this modification
    Value      interface{}   // The modification value
    Target     string        // Optional: specific target aspect
}

// Effects provide modifications
type ModifierProvider interface {
    GetModifications(ctx context.Context, action Action) []Modification
}
```

### Rulebook-Defined Pipelines

```go
// rulebooks/dnd5e/combat/pipeline.go
var AttackPipeline = Pipeline{
    Stages: []Stage{
        StageBase,       // Base weapon damage, ability modifiers
        StageEquipment,  // Magic weapons, items
        StageFeatures,   // Class features (Rage, Sneak Attack)
        StageSpells,     // Active spell effects (Bless, Hex)
        StageConditions, // Status conditions (Exhaustion, Poisoned)
        StageSituational,// Advantage/disadvantage, cover
        StageFinal,      // Critical hits, final adjustments
    },
}

var SavingThrowPipeline = Pipeline{
    Stages: []Stage{
        StageBase,        // Ability modifier
        StageProficiency, // Proficiency bonus if proficient
        StageFeatures,    // Class features (Aura of Protection)
        StageSpells,      // Spell effects (Bless, Resistance)
        StageConditions,  // Conditions affecting saves
        StageItems,       // Magic items (Cloak of Protection)
    },
}
```

### Effect Implementation

```go
// Effects declare their stage
type RageEffect struct {
    *effects.Core
    level int
}

func (r *RageEffect) GetModifications(ctx context.Context, action Action) []Modification {
    if action.Type != ActionTypeAttack {
        return nil
    }
    
    return []Modification{
        {
            Stage:  StageFeatures,
            Type:   ModifierDamageBonus,
            Source: "rage",
            Value:  r.calculateDamageBonus(),
        },
        {
            Stage:  StageFeatures,
            Type:   ModifierAdvantage,
            Source: "rage",
            Target: "strength_check",
        },
    }
}

// Effects still use events for state tracking
func (r *RageEffect) Subscribe(bus EventBus) {
    bus.Subscribe("combat.attack", func(e Event) {
        if e.Attacker == r.owner {
            r.hasActedHostile = true
        }
    })
    
    bus.Subscribe("turn.end", func(e Event) {
        if !r.hasActedHostile {
            r.Remove() // End rage
        }
        r.hasActedHostile = false
    })
}
```

### Pipeline Processing

```go
func (p *Pipeline) Calculate(ctx context.Context, base Value, action Action) Result {
    result := base
    history := []AppliedModification{}
    
    // Gather modifications from all active effects
    modifications := []Modification{}
    for _, effect := range p.effectManager.GetActive() {
        if provider, ok := effect.(ModifierProvider); ok {
            mods := provider.GetModifications(ctx, action)
            modifications = append(modifications, mods...)
        }
    }
    
    // Process in pipeline stage order
    for _, stage := range p.Stages {
        stageMods := filterByStage(modifications, stage)
        
        for _, mod := range stageMods {
            oldValue := result
            result = applyModification(result, mod)
            
            // Track modification history
            history = append(history, AppliedModification{
                Stage:    stage,
                Modifier: mod,
                Before:   oldValue,
                After:    result,
            })
        }
        
        // Optional: Publish stage completion for debugging/UI
        p.bus.Publish(StageCompleteEvent{
            Stage:   stage,
            Result:  result,
            Applied: stageMods,
        })
    }
    
    return Result{
        Value:   result,
        History: history,
    }
}
```

## Consequences

### Positive

- **Predictable Order**: Modifications always apply in rulebook-defined order
- **Debugging Support**: Can inspect modification history to see what changed when
- **Maintains Decoupling**: Event system still handles reactive behavior
- **Type Safety**: Modifications are typed, reducing runtime errors
- **Testability**: Can test pipelines without full event bus
- **Selective Loading**: Can query effects for specific modification types
- **Game-Accurate**: Matches how tabletop games actually process modifiers

### Negative

- **Additional Complexity**: Two systems (events + pipelines) instead of one
- **Migration Effort**: Existing effects need updating to support pipelines
- **Memory Overhead**: Tracking modification history uses more memory
- **Learning Curve**: Developers must understand both patterns

### Neutral

- **Explicit Stage Declaration**: Effects must declare their pipeline stage
- **Rulebook Responsibility**: Each rulebook must define its pipelines
- **Dual Pattern**: Some logic in event handlers, some in modification providers

## Implementation Plan

1. **Phase 1: Core Infrastructure**
   - Create pipeline types and interfaces
   - Implement modification tracking
   - Add pipeline processor

2. **Phase 2: Rulebook Integration**
   - Define D&D 5e pipelines (attack, saves, checks)
   - Update rage feature to use pipelines
   - Create examples for each stage type

3. **Phase 3: Effect Migration**
   - Update existing effects to provide modifications
   - Maintain backward compatibility during transition
   - Document migration guide

4. **Phase 4: Advanced Features**
   - Add modification queries ("what modifies my attack?")
   - Implement stacking rules (same source doesn't stack)
   - Add pipeline visualization for debugging

## Alternatives Considered

1. **Pure Pull-Based System**: Would lose valuable event decoupling
2. **Priority-Only Events**: Doesn't solve the fundamental ordering problem
3. **Hardcoded Modification Order**: Not flexible enough for different games
4. **Single Monolithic Pipeline**: Wouldn't support different action types

## Success Metrics

- Rage feature implementation reduces from 300+ lines to ~50 lines
- Modification order is consistent across multiple test runs
- Can query "what modified this value" and get complete history
- New effects can be added without changing pipeline code
- Combat calculations match tabletop game rules exactly

## References

- Journey 024: Effect Modification Pipeline exploration
- PR #221: Abandoned rage implementation that revealed these issues
- ADR-0023: Core types and rulebook implementation patterns
- Issue #224: Refactor features to use effects.Core