# ADR-0025: Pipeline Architecture - Everything is a Pipeline

## Status
Accepted

## Context

What started as an attempt to fix effect modifications in the rage feature (PR #221) led to a fundamental architectural discovery documented across several journey entries:

- Journey 024: Discovered need for modification pipelines
- Journey 025: Worked through complex D&D mechanics  
- Journey 026: Realized "pipelines all the way down"
- Journey 027: Understood that rulebooks own pipelines

The evolution:
1. Started with: "Effects modifying values through events is messy"
2. Thought: "Add pipelines for predictable calculations"
3. Realized: "Pipelines can nest - damage triggers concentration saves"
4. Breakthrough: "EVERYTHING is a typed pipeline"
5. Final insight: "Rulebooks define pipelines, toolkit just executes them"

## Decision

We will adopt a **Pipeline-First Architecture** where:

1. **Every game mechanic is a typed pipeline** transforming input to output
2. **Pipelines can nest** - one pipeline can trigger others
3. **Rulebooks own their pipelines** - each game defines its rules as pipelines
4. **Toolkit provides infrastructure** - execution, composition, context management
5. **Effects provide pipeline stages** - not event handlers

This supersedes ADR-0024 (Modification Pipeline Pattern) by expanding the concept beyond just modifications to the entire game architecture.

## Architecture

### Core Pipeline Types

```go
// Core pipeline abstraction
type Pipeline[TInput, TOutput any] interface {
    Process(ctx PipelineContext, input TInput) (TOutput, error)
}

// Stages transform values
type PipelineStage[T any] interface {
    Name() string
    Process(ctx PipelineContext, value T) (T, error)
}

// Context threads through all pipelines
type PipelineContext struct {
    // Game state
    CurrentTurn   EntityID
    Round         int
    InitiatorTurn EntityID
    
    // Nesting support
    Parent        *PipelineContext
    Depth         int
    CallStack     []string
    
    // Interrupt handling
    InterruptPoints map[string][]Interrupt
    
    // Decision tracking
    Decisions     []Decision
    
    // Modification history
    Modifications []AppliedModification
}
```

### Pipeline Composition

```go
// Sequential composition
type SequentialPipeline[T any] struct {
    Stages []PipelineStage[T]
}

// Nested composition
type NestablePipeline[T, U any] interface {
    Pipeline[T, U]
    Trigger(other Pipeline) (result any, err error)
}

// Interruptible pipelines
type InterruptiblePipeline[T, U any] struct {
    Pipeline[T, U]
    InterruptPoints []InterruptPoint
}
```

### Rulebook-Defined Pipelines

```go
// rulebooks/dnd5e/pipelines/combat.go
package dnd5e

var AttackPipeline = pipeline.New[AttackInput, AttackOutput](
    BaseRollStage{},        // d20 + prof + ability
    EquipmentStage{},       // Magic weapons
    ClassFeatureStage{},    // Rage, Sneak Attack
    SpellEffectStage{},     // Bless, Bane
    AdvantageStage{},       // Advantage/Disadvantage
    CriticalStage{},        // Nat 20s and 1s
)

var SavingThrowPipeline = pipeline.New[SaveInput, SaveOutput](
    AbilityModifierStage{},
    ProficiencyStage{},
    ClassFeatureStage{},    // Aura of Protection
    SpellEffectStage{},     // Bless, Resistance
    ItemBonusStage{},       // Cloak of Protection
)

var ConcentrationPipeline = pipeline.New[ConcentrationInput, ConcentrationOutput](
    CalculateDCStage{},     // 10 or half damage
    TriggerSaveStage{},     // Triggers SavePipeline!
    CheckResultStage{},     // Pass/fail
)
```

### Nested Pipeline Example

```go
// Attack triggers damage triggers concentration
func (a *AttackPipeline) Process(ctx PipelineContext, input AttackInput) (AttackOutput, error) {
    output := a.processStages(ctx, input)
    
    if output.Hit {
        // Nested pipeline execution
        damageResult, _ := DamagePipeline.Process(ctx.Nest("damage"), DamageInput{
            Amount: output.RolledDamage,
            Type:   output.DamageType,
            Target: input.Target,
        })
        output.DamageDealt = damageResult.FinalDamage
    }
    
    return output, nil
}

func (d *DamagePipeline) Process(ctx PipelineContext, input DamageInput) (DamageOutput, error) {
    output := d.processStages(ctx, input)
    
    // Check for concentration
    if IsConcentrating(input.Target) {
        // Another nested pipeline!
        concResult, _ := ConcentrationPipeline.Process(ctx.Nest("concentration"), ConcentrationInput{
            Caster: input.Target,
            Damage: output.FinalDamage,
        })
        
        if concResult.Broken {
            // Effect ends
        }
    }
    
    return output, nil
}
```

### Effects as Stage Providers

```go
// Instead of event handlers, effects provide pipeline stages
type RageEffect struct {
    level int
    active bool
}

func (r *RageEffect) GetPipelineStages() map[PipelineType]PipelineStage {
    return map[PipelineType]PipelineStage{
        AttackPipeline: &RageAttackStage{r},
        DamagePipeline: &RageDamageResistanceStage{r},
    }
}

type RageAttackStage struct {
    rage *RageEffect
}

func (s *RageAttackStage) Process(ctx PipelineContext, attack AttackValue) (AttackValue, error) {
    if s.rage.active && attack.IsMelee() {
        attack.DamageBonus += s.rage.DamageBonus()
        attack.Advantage = attack.IsStrengthBased()
    }
    return attack, nil
}
```

## Implementation Strategy

### Phase 1: Core Infrastructure
- Pipeline interfaces and types
- PipelineContext implementation
- Stage composition patterns
- Nesting support

### Phase 2: Basic Pipelines
- Attack pipeline
- Damage pipeline
- Saving throw pipeline
- Skill check pipeline

### Phase 3: Complex Interactions
- Concentration (nested saves)
- Reactions (interrupts)
- Counterspell (cancellation)
- Death saves (state tracking)

### Phase 4: Migration
- Convert rage to pipeline stages
- Convert other effects
- Remove old event-based modifications

## Consequences

### Positive

- **Type Safety**: Every pipeline has typed input/output
- **Testability**: Each pipeline testable in isolation
- **Composability**: Complex behavior from simple pipelines
- **Debuggability**: Full execution trace through context
- **Game Agnostic**: Toolkit doesn't know game rules
- **Reusability**: Stages shared across pipelines
- **Predictability**: Explicit execution order
- **Modularity**: New games just define new pipelines

### Negative

- **Paradigm Shift**: Completely different mental model
- **Migration Effort**: All effects need rewriting
- **Learning Curve**: Developers must understand pipelines
- **Complexity**: More abstractions to understand

### Neutral

- **Everything Changes**: This affects every game mechanic
- **Rulebook Responsibility**: Each game must define pipelines
- **Two Patterns**: Events for notifications, pipelines for calculations

## Success Metrics

- Rage implementation reduces from 300+ lines to <50 lines
- New game mechanics expressible as pipelines
- Combat calculations match tabletop rules exactly
- Full execution traces available for debugging
- Different games (D&D, Pathfinder) use same infrastructure

## The Paradigm Shift

Before: "Games are event-driven state machines"
After: "Games are collections of typed transformation pipelines"

This isn't just a refactor - it's a fundamental rethinking of how tabletop RPG rules are expressed in code.

## Examples Across Games

### D&D 5e Attack
```go
AttackPipeline[AttackInput, AttackOutput]
    → DamagePipeline[DamageInput, DamageOutput]
        → ConcentrationPipeline[ConcentrationInput, ConcentrationOutput]
            → SavePipeline[SaveInput, SaveOutput]
```

### Pathfinder 2e Attack (Different Rules!)
```go
AttackPipeline[AttackInput, AttackOutput]  // Multiple attack penalty
    → DamagePipeline[DamageInput, DamageOutput]  // Different resistances
        → PersistentDamagePipeline[...]  // Pathfinder-specific
```

### Call of Cthulhu Sanity
```go
SanityCheckPipeline[SanityInput, SanityOutput]
    → MadnessPipeline[MadnessInput, MadnessOutput]
        → BoutOfMadnessPipeline[...]
```

## The Ultimate Vision

```go
type Game interface {
    GetPipelines() map[string]Pipeline
}

type DnD5e struct{}
func (d DnD5e) GetPipelines() map[string]Pipeline {
    return map[string]Pipeline{
        "attack": AttackPipeline,
        "save": SavePipeline,
        "damage": DamagePipeline,
        // ... all D&D mechanics as pipelines
    }
}

// The toolkit just runs whatever pipelines the game provides
engine.Run(game.GetPipelines(), initialState)
```

## References

- PR #221: Rage implementation that started this journey
- Journey 024-027: The evolution of understanding
- Issues #228-231: Initial pipeline implementation tasks
- ADR-0024: Superseded modification pipeline pattern

## Note

This represents a fundamental architectural shift discovered through iterative problem-solving. We didn't set out to redesign everything - we just wanted to fix rage. The pipeline pattern emerged as the natural solution to composing game mechanics in a type-safe, testable way.

**The toolkit provides the stage. The rulebooks write the play.**