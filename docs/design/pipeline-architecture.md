# Pipeline Architecture

**Status: AUTHORITATIVE DESIGN**

*See Journey 048-055 for exploration and evolution of this design*

## Overview

Pipelines are the core abstraction for game mechanics in RPG Toolkit. They transform input to output through stages, returning data for the game server to persist. This replaces complex event-driven systems with pure, testable functions.

## Core Concepts

### Pipeline
A game mechanic expressed as a transformation:
```go
type Pipeline interface {
    GetRef() *core.Ref
    Process(ctx *Context, input any) Result
    Resume(data ContinuationData, decision any) Result
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

## Module Structure

```
pipeline/                      # Core infrastructure
├── types.go                  # Interfaces
├── context.go               # PipelineContext
├── executor.go              # Execution engine
├── registry.go              # Pipeline registry
├── continuation.go          # Suspension/resumption
└── data.go                  # Data types

rulebooks/dnd5e/             # Game implementation
├── pipelines.go            # All D&D pipelines
├── stages.go               # D&D-specific stages
└── registry.go             # Registration
```

## Execution Patterns

### 1. Sequential (Simple)
Stages execute in order:
```go
AttackPipeline := Sequential(
    D20RollStage,
    ProficiencyStage,
    ModifierStage,
    CriticalStage,
)
```

### 2. Nested (Cascading)
Pipelines trigger other pipelines:
```go
// Attack triggers damage triggers concentration
AttackPipeline → DamagePipeline → ConcentrationPipeline
```

### 3. Suspended (Decisions)
Pipelines suspend for player decisions:
```go
// Shield reaction
AttackPipeline.Process()
    ↓
    SUSPEND: "Cast Shield?"
    ↓
    Resume with decision
    ↓
    Complete
```

## Implementation Example

### D&D Attack Pipeline

```go
// rulebooks/dnd5e/pipelines.go
type AttackPipeline struct {
    stages []Stage
}

func (p *AttackPipeline) Process(ctx *Context, input any) Result {
    attack := input.(AttackInput)
    
    // Roll d20
    roll := rand.Intn(20) + 1
    total := roll + attack.Modifier
    
    // Check hit
    if total >= attack.TargetAC {
        // Trigger damage pipeline
        damageResult := ctx.Registry.Get(DamageRef).Process(
            ctx.Nest("damage"),
            DamageInput{
                Target: attack.Target,
                Amount: rollDamage(),
            },
        )
        
        // Return combined data
        return CompletedResult{
            Output: AttackOutput{Hit: true},
            Data: append(
                []Data{LogData{Message: "Attack hits!"}},
                damageResult.GetData()...,
            ),
        }
    }
    
    return CompletedResult{
        Output: AttackOutput{Hit: false},
        Data: []Data{LogData{Message: "Attack misses!"}},
    }
}
```

### Registration

```go
// rulebooks/dnd5e/registry.go
func RegisterPipelines(r *pipeline.Registry) {
    r.Register(AttackRef, &AttackPipelineFactory{})
    r.Register(DamageRef, &DamagePipelineFactory{})
    r.Register(SaveRef, &SavePipelineFactory{})
}
```

## Server Integration

### Encounter Management

```go
type Encounter struct {
    ID            string
    Continuations map[string]ContinuationData  // Suspended pipelines
    Entities      []Entity
    Round         int
}

func (s *GameServer) ExecuteAction(encounterID string, action Action) Response {
    encounter := s.encounters[encounterID]
    
    // Get pipeline by ref
    pipeline := s.registry.Get(action.PipelineRef)
    
    // Execute
    result := pipeline.Process(newContext(), action.Input)
    
    if result.IsComplete() {
        // Apply data
        for _, data := range result.GetData() {
            s.store.Apply(data)
        }
        return CompletedResponse{Output: result.GetOutput()}
    } else {
        // Save continuation
        contID := uuid.New().String()
        encounter.Continuations[contID] = result.GetContinuation()
        return SuspendedResponse{
            ContinuationID: contID,
            Decision:       result.GetDecision(),
        }
    }
}
```

### Resume After Decision

```go
func (s *GameServer) ResumeAction(encounterID, contID string, decision Decision) Response {
    encounter := s.encounters[encounterID]
    cont := encounter.Continuations[contID]
    
    // Get pipeline and resume
    pipeline := s.registry.Get(cont.PipelineRef)
    result := pipeline.Resume(cont, decision)
    
    // Apply data and handle result...
}
```

## Combat Room Demo

The simplest example showing the power of pipelines:

```go
// Create encounter with dummy
encounter := NewEncounter()
encounter.AddEntity(CombatDummy{HP: 100, AC: 10})

// Player attacks
result := attackPipeline.Process(ctx, AttackInput{
    Attacker: "player",
    Target:   "dummy",
    Modifier: 5,
})

// Apply results
for _, data := range result.GetData() {
    if hpData, ok := data.(*HPData); ok {
        dummy.HP += hpData.Change  // Damage applied
    }
}
```

## Key Benefits

1. **Pure Functions** - Pipelines have no side effects
2. **Testable** - Easy to test in isolation
3. **Composable** - Complex mechanics from simple pipelines
4. **Traceable** - Full execution path in context
5. **Suspendable** - Natural async support for decisions
6. **Game Agnostic** - Toolkit doesn't know game rules

## Design Principles

1. **Rulebooks Own Everything** - Each game defines its pipelines
2. **Data Not Side Effects** - Pipelines return changes to apply
3. **Explicit Over Implicit** - Clear execution flow
4. **No Premature Abstraction** - Extract common patterns later

## Migration from Events

Before (events):
```go
// Complex event chains, scattered state mutations
bus.Publish(AttackEvent{...})
// Triggers DamageEvent, ConcentrationEvent, etc.
// State changes happen in various handlers
```

After (pipelines):
```go
// Clear, traceable execution
result := attackPipeline.Process(ctx, input)
applyData(result.GetData())
```

## References

- ADR-0025: Pipeline Architecture Decision
- Journey 024-027: Evolution from events to pipelines
- Journey 048-055: Pipeline design exploration
- Issue #240: Implementation tracking