# Pipeline Architecture Layout

**Status: ARCHITECTURAL OVERVIEW**

## Where Pipeline Lives

### Core Infrastructure (rpg-toolkit provides)

```
rpg-toolkit/
├── core/
│   ├── pipeline/                    # PIPELINE INFRASTRUCTURE LIVES HERE
│   │   ├── types.go                # Core interfaces
│   │   ├── pipeline.go             # Base pipeline implementation
│   │   ├── stage.go                # Stage interface and helpers
│   │   ├── context.go              # PipelineContext
│   │   ├── executor.go             # Execution engine
│   │   ├── composer.go             # Pipeline composition helpers
│   │   ├── continuation.go         # Suspension/resumption
│   │   └── data.go                 # Data pattern types
│   │
│   ├── ref.go                      # TypedRef (already exists)
│   └── entity.go                   # Entity interface
│
├── events/                         # Event bus (triggers pipelines)
│   ├── bus.go                      # Event infrastructure
│   └── typed.go                    # TypedRef integration
│
└── mechanics/                      # REUSABLE PIPELINE STAGES
    ├── dice/                       # Dice-related stages
    │   ├── roll_stage.go          # RollD20Stage, RollDiceStage
    │   └── advantage_stage.go     # AdvantageStage
    │
    ├── modifiers/                  # Modifier stages
    │   ├── ability_stage.go        # AbilityModifierStage
    │   ├── proficiency_stage.go    # ProficiencyStage
    │   └── bonus_stage.go          # Generic bonus stages
    │
    └── conditions/                 # Condition-checking stages
        ├── status_stage.go         # Check status conditions
        └── requirement_stage.go    # Check requirements
```

### Rulebook Implementation (games define their pipelines)

```
rulebooks/
└── dnd5e/                          # D&D 5E RULEBOOK
    ├── pipelines/                  # GAME-SPECIFIC PIPELINES
    │   ├── attack.go              # AttackPipeline definition
    │   ├── damage.go              # DamagePipeline (knows about concentration!)
    │   ├── saves.go               # SavePipeline
    │   ├── death.go               # DeathSavePipeline
    │   ├── concentration.go       # ConcentrationPipeline
    │   └── skills.go              # SkillCheckPipeline
    │
    ├── stages/                     # D&D-SPECIFIC STAGES
    │   ├── critical_stage.go       # D&D crit rules (nat 20)
    │   ├── advantage_stage.go      # D&D advantage/disadvantage
    │   ├── rage_stage.go          # Rage modifications
    │   └── sneak_attack_stage.go  # Rogue sneak attack
    │
    └── registry.go                 # Register all D&D pipelines
```

## How It Works Together

### 1. Core Provides Infrastructure

```go
// core/pipeline/types.go
package pipeline

// The fundamental abstraction
type Pipeline[TInput, TOutput any] interface {
    Process(ctx *Context, input TInput) Result[TOutput]
}

// Results can be completed or suspended
type Result[T any] interface {
    IsComplete() bool
    GetOutput() (T, error)
    GetData() []Data
}

// Stages transform values
type Stage[T any] interface {
    Name() string
    Process(ctx *Context, value T) (T, error)
}

// Context threads through execution
type Context struct {
    Round       int
    CurrentTurn string
    Parent      *Context
    Depth       int
    CallStack   []string
}
```

### 2. Mechanics Provides Reusable Stages

```go
// mechanics/dice/roll_stage.go
package dice

// Reusable by any game that rolls d20s
type RollD20Stage struct {
    Modifier int
}

func (s *RollD20Stage) Process(ctx *pipeline.Context, input RollInput) (RollInput, error) {
    input.Roll = rollD20()
    input.Total = input.Roll + s.Modifier
    return input, nil
}
```

### 3. Rulebook Defines Game Pipelines

```go
// rulebooks/dnd5e/pipelines/attack.go
package pipelines

import (
    "rpg-toolkit/core/pipeline"
    "rpg-toolkit/mechanics/dice"
    "rpg-toolkit/mechanics/modifiers"
    "rulebooks/dnd5e/stages"
)

// D&D 5e attack pipeline
var AttackPipeline = pipeline.Sequential[AttackInput, AttackOutput](
    &dice.RollD20Stage{},              // From mechanics (reusable)
    &modifiers.ProficiencyStage{},     // From mechanics (reusable)
    &modifiers.AbilityStage{},         // From mechanics (reusable)
    &stages.DnDAdvantageStage{},       // D&D specific!
    &stages.DnDCriticalStage{},        // D&D specific!
)

// D&D 5e damage pipeline KNOWS about concentration
var DamagePipeline = &DamageWithConcentration{
    base: pipeline.Sequential[DamageInput, DamageOutput](
        &stages.RollDamageStage{},
        &stages.ResistanceStage{},
        &stages.VulnerabilityStage{},
    ),
}

type DamageWithConcentration struct {
    base pipeline.Pipeline[DamageInput, DamageOutput]
}

func (d *DamageWithConcentration) Process(ctx *pipeline.Context, input DamageInput) pipeline.Result[DamageOutput] {
    // Process base damage
    result := d.base.Process(ctx, input)
    
    // D&D RULE: Damage triggers concentration checks!
    if input.Target.IsConcentrating() {
        concResult := ConcentrationPipeline.Process(
            ctx.Nest("concentration"),
            ConcentrationInput{
                Entity: input.Target,
                Damage: result.GetOutput().FinalDamage,
            },
        )
        
        // Handle suspension if needed
        if !concResult.IsComplete() {
            return pipeline.Suspended[DamageOutput]{
                // ... suspension data
            }
        }
    }
    
    return result
}
```

### 4. Features Provide Pipeline Stages

```go
// rulebooks/dnd5e/features/rage.go
package features

// Rage provides stages for pipelines
type RageFeature struct {
    active bool
    level  int
}

func (r *RageFeature) GetPipelineStages() map[string]pipeline.Stage {
    return map[string]pipeline.Stage{
        "attack.damage": &RageDamageStage{rage: r},
        "damage.resistance": &RageResistanceStage{rage: r},
    }
}

type RageDamageStage struct {
    rage *RageFeature
}

func (s *RageDamageStage) Process(ctx *pipeline.Context, attack AttackValue) (AttackValue, error) {
    if s.rage.active && attack.IsMelee() {
        attack.DamageBonus += s.rage.GetDamageBonus()
    }
    return attack, nil
}
```

## How Rulebooks Leverage It

### 1. Define Their Game's Pipelines

```go
// rulebooks/dnd5e/registry.go
package dnd5e

func RegisterPipelines(engine *pipeline.Engine) {
    engine.Register("attack", AttackPipeline)
    engine.Register("damage", DamagePipeline)
    engine.Register("save", SavePipeline)
    engine.Register("death_save", DeathSavePipeline)
    engine.Register("concentration", ConcentrationPipeline)
}
```

### 2. Implement Game-Specific Rules

```go
// D&D 5e has death saves
var DeathSavePipeline = pipeline.Sequential[DeathSaveInput, DeathSaveOutput](
    &dice.RollD20Stage{},
    &stages.NaturalTwentyStage{},     // D&D: nat 20 = 1 HP
    &stages.NaturalOneStage{},         // D&D: nat 1 = 2 failures
    &stages.CompareToTenStage{},       // D&D: 10+ succeeds
    &stages.TrackSavesStage{},         // D&D: 3 success/fail
)

// Pathfinder has dying value instead
var DyingPipeline = pipeline.Sequential[DyingInput, DyingOutput](
    &stages.CheckWoundedStage{},       // PF2e: wounded increases dying
    &stages.IncreaseDyingStage{},      // PF2e: dying is a value
    &stages.RecoveryCheckStage{},      // PF2e: DC 10 + dying
)
```

### 3. Mix Core and Custom Stages

```go
var SkillCheckPipeline = pipeline.Sequential[SkillInput, SkillOutput](
    &dice.RollD20Stage{},               // From core mechanics
    &modifiers.AbilityStage{},          // From core mechanics
    &modifiers.ProficiencyStage{},      // From core mechanics
    &stages.DnDExpertiseStage{},        // D&D specific: double prof
    &stages.DnDReliableTalentStage{},   // D&D specific: min 10
)
```

## The Event → Pipeline Connection

```go
// Events trigger pipelines
eventBus.Subscribe(DamageIntentRef, func(ctx context.Context, e *DamageIntentEvent) error {
    // Event triggers pipeline
    result := DamagePipeline.Process(
        pipeline.NewContext(),
        DamageInput{
            Source: e.Source,
            Target: e.Target,
            Amount: e.Amount,
        },
    )
    
    // Apply returned data
    for _, data := range result.GetData() {
        gameServer.Apply(data)
    }
    
    return nil
})
```

## The Key Separation

**Toolkit (core/pipeline):**
- HOW to execute pipelines
- HOW to compose stages
- HOW to suspend/resume
- HOW to handle context

**Rulebooks (rulebooks/dnd5e):**
- WHAT pipelines exist
- WHAT stages they contain
- WHEN to trigger nested pipelines
- WHAT game rules apply

**Mechanics (mechanics/):**
- Reusable stages any game can use
- Dice rolling
- Basic modifiers
- Common patterns

This separation means:
- New games just define pipelines
- Core never needs game knowledge
- Games can share common mechanics
- Each game fully controls its rules

The pipeline infrastructure is the engine. The rulebooks write the instructions.