# Pipeline Module

The pipeline module provides the core infrastructure for expressing game mechanics as pure, composable transformations.

## Overview

Pipelines replace complex event-driven systems with simple, testable functions that transform typed input to typed output and return data for the game server to persist.

## Core Concepts

### Pipeline
A game mechanic expressed as a type-safe transformation:
```go
type Pipeline[I, O any] interface {
    GetRef() *core.Ref
    Process(ctx context.Context, input I) Result[O]
    Resume(continuation ContinuationData, decision any) Result[O]
}
```
- Takes typed input, produces typed output
- Returns data (state changes) to apply
- No side effects - pure functions

### Stages
Individual transformation steps that can be composed:
- Each stage transforms a value
- Stages can be chained sequentially
- Stages can return both values and data

### Data Pattern
Pipelines don't mutate state directly. They return data describing what changed:
- `HPData` - Hit point changes
- `LogData` - Combat log entries
- `ConditionData` - Status effect changes
- `ResourceData` - Resource consumption (spell slots, etc.)

## Usage

### Simple Typed Pipeline

```go
// Create stages
addFive := pipeline.NewStage("add-five", func(ctx context.Context, input any) (any, error) {
    value := input.(int)
    return value + 5, nil
})

double := pipeline.NewStage("double", func(ctx context.Context, input any) (any, error) {
    value := input.(int)
    return value * 2, nil
})

// Chain stages into a typed pipeline
p := pipeline.Sequential[int, int](ref, addFive, double)

// Execute: (10 + 5) * 2 = 30
result := p.Process(context.Background(), 10)
fmt.Println(result.GetOutput()) // 30 (type-safe int)
```

### Typed Attack Pipeline

```go
// Define typed input/output
type AttackInput struct {
    Attacker string
    Target   string
    Bonus    int
}

type AttackOutput struct {
    Hit    bool
    Damage int
}

// Create attack pipeline
attackPipeline := pipeline.Sequential[AttackInput, AttackOutput](ref,
    pipeline.NewStage("roll", func(ctx context.Context, input any) (any, error) {
        attack := input.(AttackInput)
        roll := dice.Roll("1d20") + attack.Bonus
        hit := roll >= targetAC
        
        var damage int
        if hit {
            damage = dice.Roll("1d8") + attack.Bonus
        }
        
        return AttackOutput{
            Hit:    hit,
            Damage: damage,
        }, nil
    }),
)

// Type-safe execution
result := attackPipeline.Process(ctx, AttackInput{
    Attacker: "fighter",
    Target:   "goblin",
    Bonus:    5,
})

output := result.GetOutput() // AttackOutput, not any!
if output.Hit {
    fmt.Printf("Hit for %d damage!\n", output.Damage)
}
```

### Pipeline with Data

```go
// Stage that returns data
damageStage := pipeline.NewStage("damage", func(ctx context.Context, input any) (any, error) {
    damage := input.(int)
    
    return pipeline.StageResult{
        Value: damage,
        Data: []pipeline.Data{
            &pipeline.HPData{
                EntityID: "goblin",
                Amount:   -damage,
            },
            &pipeline.LogData{
                Message: fmt.Sprintf("Goblin takes %d damage", damage),
            },
        },
    }, nil
})

// Execute and get data
result := pipeline.Sequential[int, int](ref, damageStage).Process(ctx, 10)

// Apply returned data
for _, data := range result.GetData() {
    store.Apply(data)
}
```

### Registry Pattern with Type Safety

```go
// Create registry
registry := pipeline.NewRegistry()

// Register typed pipeline factories
registry.Register(attackRef, pipeline.Func[AttackInput, AttackOutput](func() pipeline.Pipeline[AttackInput, AttackOutput] {
    return createAttackPipeline()
}))

// Get pipeline with proper types
attackPipeline, err := pipeline.Get[AttackInput, AttackOutput](registry, attackRef)
if err != nil {
    return err
}

// Type-safe execution
result := attackPipeline.Process(ctx, AttackInput{...})
output := result.GetOutput() // AttackOutput, compile-time checked!
```

## Why Pipelines?

### Before (Events)
```go
// Complex event chains, hard to test, scattered state mutations
bus.Publish(AttackEvent{...})
// Triggers DamageEvent, ConcentrationEvent, SaveEvent...
// State changes happen in various handlers
// Hard to trace execution flow
// No type safety - everything is interface{}
```

### After (Typed Pipelines)
```go
// Simple, testable, traceable, type-safe
result := attackPipeline.Process(ctx, AttackInput{
    Attacker: "fighter",
    Target:   "goblin",
    Bonus:    5,
})
// Compile-time type checking!
if result.GetOutput().Hit {
    for _, data := range result.GetData() {
        store.Apply(data)
    }
}
```

## Benefits

1. **Type Safety** - Compile-time checking of inputs and outputs
2. **Pure Functions** - No side effects, easy to test
3. **Composable** - Build complex mechanics from simple stages
4. **Traceable** - Clear execution path
5. **Game Agnostic** - Infrastructure doesn't know game rules

## Future Features

- **Suspension/Resumption** - Pause for player decisions
- **Nested Pipelines** - Type-safe pipeline composition
- **Interrupt Points** - Reactions and timing windows

## Design Philosophy

Pipelines provide type-safe infrastructure. Rulebooks define the game mechanics with proper types. The toolkit doesn't know about D&D, Pathfinder, or any specific game - it just knows how to execute typed pipelines.