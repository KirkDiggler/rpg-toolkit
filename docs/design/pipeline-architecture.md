# Pipeline Architecture

**Status: AUTHORITATIVE DESIGN - INITIAL IMPLEMENTATION**

## Overview

Pipelines are the core abstraction for game mechanics in RPG Toolkit. They transform input to output through stages, returning data for the game server to persist. This replaces complex event-driven systems with pure, testable functions.

## Core Concepts

### Pipeline
A game mechanic expressed as a transformation:
```go
type Pipeline interface {
    GetRef() *core.Ref
    Process(ctx context.Context, input any) Result
    Resume(continuation ContinuationData, decision any) Result
}
```

### Result
Pipelines return results that can be complete or suspended:
```go
type Result interface {
    IsComplete() bool
    GetData() []Data      // State changes to apply
    GetOutput() any        // Pipeline output
}
```

### Data Pattern
Pipelines don't cause side effects - they return data:
```go
// Pipeline returns what happened
result := attackPipeline.Process(ctx, input)

// Server applies the changes
for _, data := range result.GetData() {
    store.Apply(data)  // HP changes, conditions, etc.
}
```

## Implementation Status

### ✅ Phase 1: Core Infrastructure (COMPLETE)
- [x] Basic types and interfaces
- [x] Sequential executor for chaining stages
- [x] Pipeline registry with ref-based lookup
- [x] Data types (HP, Log, Condition, Resource)
- [x] Context with nesting support
- [x] Tests demonstrating the pattern

### 🚧 Phase 2: Combat Demo (NEXT)
- [ ] Simple D&D attack pipeline
- [ ] Combat dummy encounter
- [ ] Integration with game server

### 📋 Phase 3: Advanced Features (FUTURE)
- [ ] Suspension/resumption for decisions
- [ ] Nested pipeline execution
- [ ] Complex combat mechanics

## Module Structure

```
pipeline/                     # Core infrastructure
├── types.go                 # Pipeline, Result, Stage interfaces
├── executor.go              # Sequential execution
├── registry.go              # Pipeline registry
├── data.go                  # Data types (HP, Log, etc.)
├── result.go                # Result implementations
└── pipeline_test.go         # Tests

rulebooks/dnd5e/             # Game implementation (NEXT)
├── pipelines.go            # D&D pipeline definitions
├── stages.go               # D&D-specific stages
└── registry.go             # Registration
```

## Usage Example

```go
// Create and register a pipeline
attackRef, _ := core.ParseString("dnd5e:pipeline:attack")
registry.Register(attackRef, PipelineFunc(func() Pipeline {
    return Sequential(attackRef,
        rollStage,      // Roll d20
        modifierStage,  // Add modifiers
        compareStage,   // Check vs AC
        damageStage,    // Calculate damage
    )
}))

// Execute pipeline
pipeline, _ := registry.Get(attackRef)
result := pipeline.Process(ctx, AttackInput{
    Attacker: "player",
    Target:   "goblin",
    Modifier: 5,
})

// Apply returned data
for _, data := range result.GetData() {
    fmt.Println(data) // "goblin takes 8 damage"
}
```

## Key Benefits

1. **Pure Functions** - Pipelines have no side effects
2. **Testable** - Easy to test in isolation
3. **Composable** - Complex mechanics from simple stages
4. **Traceable** - Full execution path in context
5. **Game Agnostic** - Toolkit doesn't know game rules

## Design Principles

1. **Rulebooks Own Everything** - Each game defines its pipelines
2. **Data Not Side Effects** - Pipelines return changes to apply
3. **Start Simple** - Basic features first, complexity later
4. **No Premature Abstraction** - Extract patterns when needed

## Migration from Events

The combat room demo was extremely complicated with events triggering events and scattered state mutations. With pipelines:

Before:
```go
// Complex event chains, hard to trace
bus.Publish(AttackEvent{...})
// Triggers damage, conditions, saves...
// State changes in various handlers
```

After:
```go
// Clear, simple execution
result := attackPipeline.Process(ctx, input)
applyData(result.GetData())
```

## Next Steps

1. Create simple D&D attack pipeline in `rulebooks/dnd5e`
2. Wire up combat room demo
3. Compare complexity to event-based approach

## References

- Issue #240: Pipeline implementation tracking
- PR #239: Event bus improvements that led to this design