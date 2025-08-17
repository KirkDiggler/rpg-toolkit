# Simplified Pipeline Structure

**Status: DESIGN REFINEMENT**

## The Problem
Too many packages with "pipeline" in the name is confusing!

## Simplified Structure

```
rpg-toolkit/
├── core/
│   ├── go.mod
│   ├── ref.go           # Already exists
│   ├── entity.go        # Already exists
│   └── flow.go          # Pipeline interfaces (renamed!)
│
├── flow/                 # Pipeline implementation (better name!)
│   ├── go.mod
│   ├── executor.go      # Execution engine
│   ├── registry.go      # Pipeline registry
│   ├── continuation.go  # Suspension/resumption
│   └── builder.go       # Helper functions
│
├── stages/              # Reusable stages (cleaner than mechanics!)
│   ├── go.mod
│   ├── dice.go         # Dice rolling stages
│   ├── modifiers.go    # Modifier stages
│   └── conditions.go   # Condition stages
│
└── rulebooks/
    └── dnd5e/
        ├── go.mod
        ├── flows.go     # All D&D pipelines in one place
        └── stages.go    # D&D-specific stages
```

## Core: Just the Interfaces

```go
// core/flow.go - Clean, simple interfaces
package core

// Flow represents a game mechanic transformation
type Flow interface {
    GetRef() *Ref
    Process(ctx *FlowContext, input any) FlowResult
    Resume(data []byte, decision any) FlowResult
}

// FlowResult can be complete or suspended
type FlowResult interface {
    IsComplete() bool
    GetData() []Data
}

// Stage transforms values in a flow
type Stage interface {
    Name() string
    Process(ctx *FlowContext, value any) (any, error)
}

// FlowContext carries execution state
type FlowContext struct {
    Round       int
    CurrentTurn string
    Parent      *FlowContext
    Depth       int
    CallStack   []string
}
```

## Flow: The Implementation

```go
// flow/executor.go - The engine
package flow

import "github.com/KirkDiggler/rpg-toolkit/core"

// Sequential creates a flow from stages
func Sequential(ref *core.Ref, stages ...core.Stage) core.Flow {
    return &sequentialFlow{
        ref:    ref,
        stages: stages,
    }
}

// Registry manages all flows
type Registry struct {
    flows map[string]FlowFactory
}

// Register a flow
func (r *Registry) Register(ref *core.Ref, factory FlowFactory) {
    r.flows[ref.String()] = factory
}
```

## Stages: Reusable Components

```go
// stages/dice.go - Common dice stages
package stages

import "github.com/KirkDiggler/rpg-toolkit/core"

// D20Roll is used by many games
type D20Roll struct {
    Modifier int
}

func (s *D20Roll) Name() string { return "d20-roll" }

func (s *D20Roll) Process(ctx *core.FlowContext, input any) (any, error) {
    // Roll d20, add modifier
    return rollD20() + s.Modifier, nil
}

// Advantage for games that use it
type Advantage struct{}

func (s *Advantage) Process(ctx *core.FlowContext, input any) (any, error) {
    // Roll twice, take higher
    return max(rollD20(), rollD20()), nil
}
```

## Rulebook: Game Implementation

```go
// rulebooks/dnd5e/flows.go - All D&D flows in ONE file
package dnd5e

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/flow"
    "github.com/KirkDiggler/rpg-toolkit/stages"
)

// Flow refs
var (
    AttackRef = core.MustParseString("dnd5e:flow:attack")
    DamageRef = core.MustParseString("dnd5e:flow:damage")
    SaveRef   = core.MustParseString("dnd5e:flow:save")
)

// Register all D&D flows
func RegisterFlows(registry *flow.Registry) {
    registry.Register(AttackRef, &AttackFactory{})
    registry.Register(DamageRef, &DamageFactory{})
    registry.Register(SaveRef, &SaveFactory{})
}

// Attack flow
type AttackFactory struct{}

func (f *AttackFactory) Create() core.Flow {
    return flow.Sequential(AttackRef,
        &stages.D20Roll{},           // From common stages
        &stages.Proficiency{},        // From common stages
        &DnDAdvantage{},             // D&D specific (local)
        &DnDCritical{},              // D&D specific (local)
    )
}

// Damage flow with concentration check
type DamageFlow struct {
    *flow.Sequential
}

func (d *DamageFlow) Process(ctx *core.FlowContext, input any) core.FlowResult {
    result := d.Sequential.Process(ctx, input)
    
    // D&D rule: damage triggers concentration
    if needsConcentration(input) {
        // Trigger concentration flow
        concFlow := getFlow(SaveRef)
        concResult := concFlow.Process(ctx.Nest("concentration"), saveInput)
        
        // Handle suspension if needed
        if !concResult.IsComplete() {
            return flow.Suspend(result, concResult)
        }
    }
    
    return result
}
```

```go
// rulebooks/dnd5e/stages.go - D&D specific stages
package dnd5e

// D&D specific advantage rules
type DnDAdvantage struct {
    HasAdvantage    bool
    HasDisadvantage bool
}

func (s *DnDAdvantage) Process(ctx *core.FlowContext, roll any) (any, error) {
    // D&D rules: advantage/disadvantage cancel
    if s.HasAdvantage && !s.HasDisadvantage {
        return max(rollD20(), rollD20()), nil
    }
    if s.HasDisadvantage && !s.HasAdvantage {
        return min(rollD20(), rollD20()), nil
    }
    return rollD20(), nil
}

// D&D critical rules
type DnDCritical struct{}

func (s *DnDCritical) Process(ctx *core.FlowContext, attack any) (any, error) {
    a := attack.(AttackValue)
    if a.NaturalRoll == 20 {
        a.Critical = true
        a.DamageDice *= 2  // D&D: double dice on crit
    }
    return a, nil
}
```

## Benefits of Simplified Structure

### 1. Cleaner Names
- `core.Flow` instead of `pipeline.Pipeline`
- `flow` package instead of `pipeline` package
- `stages` instead of `mechanics`
- One `flows.go` per rulebook instead of `pipelines/` directory

### 2. Less Nesting
```
Before: rulebooks/dnd5e/pipelines/attack.go
After:  rulebooks/dnd5e/flows.go (all flows in one file)
```

### 3. Clear Terminology
- **Flow**: A sequence of transformations (attack flow, damage flow)
- **Stage**: A single transformation step
- **Registry**: Where flows are registered
- **Context**: Execution state

### 4. Simpler Mental Model
```go
// It's clear what this is
AttackFlow := flow.Sequential(
    D20Roll,
    Proficiency,
    Advantage,
    Critical,
)

// Process an attack
result := AttackFlow.Process(ctx, input)
```

## The Complete Picture

```
core/       → Defines Flow interface (minimal)
flow/       → Implements flow execution
stages/     → Common reusable stages
rulebooks/  → Game-specific flows and stages
```

Each rulebook is just:
- `flows.go` - All the game's flows
- `stages.go` - Game-specific stages
- `registry.go` - Registration helper

Much cleaner than having `pipeline` everywhere!