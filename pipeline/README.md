# Pipeline Module

The pipeline module provides the core infrastructure for expressing game mechanics as pure, composable transformations.

## Overview

Pipelines replace complex event-driven systems with simple, testable functions that transform input to output and return data for the game server to persist.

## Core Concepts

### Pipeline
A game mechanic expressed as a transformation:
- Takes input, produces output
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

### Simple Pipeline

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

// Chain stages into a pipeline
p := pipeline.Sequential(ref, addFive, double)

// Execute: (10 + 5) * 2 = 30
result := p.Process(context.Background(), 10)
fmt.Println(result.GetOutput()) // 30
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
result := pipeline.Sequential(ref, damageStage).Process(ctx, 10)

// Apply returned data
for _, data := range result.GetData() {
    fmt.Println(data) // "goblin takes 10 damage"
}
```

### Registry Pattern

```go
// Create registry
registry := pipeline.NewRegistry()

// Register pipelines
registry.Register(attackRef, &AttackPipelineFactory{})
registry.Register(damageRef, &DamagePipelineFactory{})

// Get and execute
pipeline, _ := registry.Get(attackRef)
result := pipeline.Process(ctx, input)
```

## Why Pipelines?

### Before (Events)
```go
// Complex event chains, hard to test, scattered state mutations
bus.Publish(AttackEvent{...})
// Triggers DamageEvent, ConcentrationEvent, SaveEvent...
// State changes happen in various handlers
// Hard to trace execution flow
```

### After (Pipelines)
```go
// Simple, testable, traceable
result := attackPipeline.Process(ctx, input)
for _, data := range result.GetData() {
    store.Apply(data)
}
```

## Benefits

1. **Pure Functions** - No side effects, easy to test
2. **Composable** - Build complex mechanics from simple stages
3. **Traceable** - Clear execution path
4. **Type-Safe** - Compile-time checking
5. **Game Agnostic** - Infrastructure doesn't know game rules

## Future Features

- **Suspension/Resumption** - Pause for player decisions
- **Nested Pipelines** - Pipelines triggering other pipelines
- **Interrupt Points** - Reactions and timing windows

## Design Philosophy

Pipelines provide the infrastructure. Rulebooks define the game mechanics. The toolkit doesn't know about D&D, Pathfinder, or any specific game - it just knows how to execute pipelines.