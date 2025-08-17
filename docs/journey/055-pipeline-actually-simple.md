# Pipeline - Actually Simple

**Status: DESIGN SIMPLIFICATION**

## The Real Question

Why separate stages from pipelines? Why not keep it all together?

## Option 1: Everything in Pipeline Package

```
rpg-toolkit/
├── core/
│   ├── ref.go
│   └── entity.go
│
├── pipeline/                    # EVERYTHING pipeline-related
│   ├── go.mod
│   ├── pipeline.go             # Interfaces and base implementation
│   ├── context.go              # PipelineContext
│   ├── registry.go             # Pipeline registry
│   ├── continuation.go         # Suspension/resumption
│   ├── data.go                 # Data pattern
│   │
│   └── stages/                 # Common stages IN the pipeline package
│       ├── dice.go             # Dice rolling stages
│       ├── modifiers.go        # Modifier stages
│       └── conditions.go       # Condition stages
│
└── rulebooks/
    └── dnd5e/
        ├── pipelines.go        # D&D pipeline definitions
        └── stages.go           # D&D-specific stages
```

**Benefits:**
- Clear that stages are part of pipeline system
- One import for all pipeline needs
- Everything related is together

```go
// Using it
import (
    "github.com/KirkDiggler/rpg-toolkit/pipeline"
    "github.com/KirkDiggler/rpg-toolkit/pipeline/stages"
)

var AttackPipeline = pipeline.Sequential(
    &stages.D20Roll{},        // Common stage
    &stages.Proficiency{},    // Common stage
    &DnDAdvantage{},         // Game-specific stage
)
```

## Option 2: Pipeline as Core Concept

Actually, if pipelines are THE core abstraction...

```
rpg-toolkit/
├── core/
│   ├── ref.go
│   ├── entity.go
│   └── pipeline.go            # Pipeline IS a core concept!
│
├── pipeline/                   # Implementation
│   ├── go.mod
│   ├── executor.go            # Execution engine
│   ├── registry.go            # Registry
│   ├── continuation.go        # Suspension
│   ├── builder.go             # Sequential, Parallel builders
│   │
│   ├── stages/                # Common stages
│   │   ├── dice.go
│   │   ├── modifiers.go
│   │   └── conditions.go
│   │
│   └── testing/               # Test helpers
│       └── mock.go
│
└── rulebooks/
    └── dnd5e/
        └── pipelines/         # Back to a directory (it's clearer!)
            ├── attack.go
            ├── damage.go
            └── stages.go      # D&D-specific stages
```

## Option 3: The Simplest Thing

Wait, why does core need to know about pipelines at all?

```
rpg-toolkit/
├── core/                      # Just Entity and Ref
│   ├── ref.go
│   └── entity.go
│
├── pipeline/                   # EVERYTHING pipeline
│   ├── go.mod (depends on core for Ref)
│   ├── types.go               # Pipeline, Stage, Context interfaces
│   ├── executor.go            # Execution
│   ├── registry.go            # Registration
│   ├── data.go                # Data pattern
│   ├── continuation.go        # Suspension
│   │
│   ├── dice.go                # Dice stages (no subdirectory!)
│   ├── modifiers.go           # Modifier stages
│   └── conditions.go          # Condition stages
│
└── rulebooks/
    └── dnd5e/
        ├── attack.go          # Attack pipeline
        ├── damage.go          # Damage pipeline  
        ├── saves.go           # Save pipelines
        └── stages.go          # ALL D&D-specific stages
```

**This is the simplest!**
- Pipeline package has everything pipeline-related
- Common stages are just files in pipeline package
- Rulebooks define their pipelines and custom stages

```go
// pipeline/types.go
package pipeline

type Pipeline[TIn, TOut any] interface {
    Process(ctx *Context, input TIn) Result[TOut]
}

type Stage[T any] interface {
    Process(ctx *Context, value T) (T, error)
}
```

```go
// pipeline/dice.go - Common dice stages
package pipeline

type D20Roll struct {
    Modifier int
}

func (s *D20Roll) Process(ctx *Context, input RollInput) (RollInput, error) {
    input.Roll = rollD20() + s.Modifier
    return input, nil
}
```

```go
// rulebooks/dnd5e/attack.go
package dnd5e

import "github.com/KirkDiggler/rpg-toolkit/pipeline"

var AttackPipeline = pipeline.Sequential(
    AttackRef,
    &pipeline.D20Roll{},      // From pipeline package
    &pipeline.Proficiency{},   // From pipeline package
    &DnDAdvantage{},          // From this package's stages.go
    &DnDCritical{},           // From this package's stages.go
)
```

## My Recommendation: Option 3

**Why?**
1. **Simplest structure** - No nested packages
2. **Clear ownership** - Pipeline package owns pipeline stuff
3. **Easy imports** - Just `pipeline` for everything
4. **Stages are just types** - D20Roll, Proficiency are just structs in the package

The insight: Stages don't need their own package/directory. They're just types that implement the Stage interface. Common ones live in the pipeline package, game-specific ones live with the game.

## Even Simpler?

Actually, do the common stages even need separate files?

```
pipeline/
├── types.go        # Interfaces
├── executor.go     # Execution
├── stages.go       # ALL common stages in one file!
└── registry.go     # Registration
```

```go
// pipeline/stages.go - All common stages
package pipeline

// Dice stages
type D20Roll struct { Modifier int }
func (s *D20Roll) Process(...) {...}

type Advantage struct {}
func (s *Advantage) Process(...) {...}

// Modifier stages
type Proficiency struct { Bonus int }
func (s *Proficiency) Process(...) {...}

type AbilityModifier struct { Ability string }
func (s *AbilityModifier) Process(...) {...}

// Condition stages
type ConditionCheck struct { Condition string }
func (s *ConditionCheck) Process(...) {...}
```

Now it's REALLY simple:
- `pipeline` package has everything
- Common stages in `stages.go`
- Games add their own stages

This feels right - not overthinking it!