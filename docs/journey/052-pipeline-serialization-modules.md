# Pipeline Serialization and Module Structure

**Status: DESIGN EXPLORATION**

## The Serialization Problem

When a pipeline suspends, we need to:
1. Serialize the continuation data to JSON
2. Store it (database, Redis, etc.)
3. Later deserialize and resume
4. Know WHICH pipeline type to reconstruct

## Pipeline References

```go
// Each pipeline needs a unique ref for serialization
type PipelineRef struct {
    *core.Ref  // Reuse existing ref system!
}

// Example refs
var AttackPipelineRef = &PipelineRef{
    Ref: core.MustParseString("dnd5e:pipeline:attack"),
}

var DamagePipelineRef = &PipelineRef{
    Ref: core.MustParseString("dnd5e:pipeline:damage"),
}
```

## Serialization Pattern

### Pipeline Registration

```go
// core/pipeline/registry.go
package pipeline

type Registry struct {
    pipelines map[string]PipelineFactory
}

type PipelineFactory interface {
    CreatePipeline() Pipeline
    LoadFromData(data json.RawMessage) (Pipeline, error)
}

// Register pipelines at startup
func (r *Registry) Register(ref *PipelineRef, factory PipelineFactory) {
    r.pipelines[ref.String()] = factory
}

// Load from continuation
func (r *Registry) LoadContinuation(cont ContinuationData) (Pipeline, error) {
    factory, exists := r.pipelines[cont.PipelineRef]
    if !exists {
        return nil, fmt.Errorf("unknown pipeline: %s", cont.PipelineRef)
    }
    return factory.LoadFromData(cont.State)
}
```

### Continuation Data with Refs

```go
// core/pipeline/continuation.go
type ContinuationData struct {
    ID           string          `json:"id"`
    PipelineRef  string          `json:"pipeline_ref"`  // "dnd5e:pipeline:attack"
    Stage        int             `json:"stage"`
    Input        json.RawMessage `json:"input"`
    Intermediate json.RawMessage `json:"intermediate"`
    Context      ContextData     `json:"context"`
}

// Serialize for storage
func (c *ContinuationData) ToJSON() ([]byte, error) {
    return json.Marshal(c)
}

// Deserialize from storage
func LoadContinuation(data []byte) (*ContinuationData, error) {
    var cont ContinuationData
    err := json.Unmarshal(data, &cont)
    return &cont, err
}
```

### Pipeline Implementation

```go
// rulebooks/dnd5e/pipelines/attack.go
package pipelines

type AttackPipelineFactory struct{}

func (f *AttackPipelineFactory) CreatePipeline() pipeline.Pipeline {
    return &AttackPipeline{
        stages: []pipeline.Stage{
            &dice.RollD20Stage{},
            &modifiers.ProficiencyStage{},
            // ...
        },
    }
}

func (f *AttackPipelineFactory) LoadFromData(data json.RawMessage) (pipeline.Pipeline, error) {
    var state AttackPipelineState
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, err
    }
    
    p := f.CreatePipeline().(*AttackPipeline)
    p.currentStage = state.Stage
    p.intermediate = state.Intermediate
    return p, nil
}

// Register at init
func init() {
    pipeline.GlobalRegistry.Register(
        AttackPipelineRef,
        &AttackPipelineFactory{},
    )
}
```

## Module Structure

### Core Module (Minimal Interfaces)

```
github.com/KirkDiggler/rpg-toolkit/core
├── go.mod
├── ref.go                 # Already exists
├── entity.go             # Already exists
└── pipeline/             # NEW - Just interfaces!
    ├── types.go          # Pipeline, Stage, Result interfaces
    ├── context.go        # PipelineContext type
    └── data.go           # Data pattern interfaces
```

```go
// core/pipeline/types.go
package pipeline

// Just the interfaces - no implementation!
type Pipeline interface {
    GetRef() *PipelineRef
    Process(ctx *Context, input any) Result
    Resume(cont ContinuationData, decision any) Result
}

type Result interface {
    IsComplete() bool
    GetData() []Data
}

type Stage interface {
    Name() string
    Process(ctx *Context, value any) (any, error)
}
```

### Pipeline Module (Implementation)

```
github.com/KirkDiggler/rpg-toolkit/pipeline
├── go.mod (depends on core)
├── executor.go           # Execution engine
├── composer.go          # Sequential, parallel composition
├── registry.go          # Pipeline registry
├── continuation.go      # Suspension/resumption
└── builder.go           # Pipeline builder helpers
```

```go
// pipeline/go.mod
module github.com/KirkDiggler/rpg-toolkit/pipeline

require (
    github.com/KirkDiggler/rpg-toolkit/core v0.1.0
)
```

### Mechanics Module (Reusable Stages)

```
github.com/KirkDiggler/rpg-toolkit/mechanics
├── go.mod (depends on core, pipeline)
├── dice/
│   ├── stages.go        # Dice rolling stages
│   └── stages_test.go
├── modifiers/
│   ├── stages.go        # Modifier stages
│   └── stages_test.go
└── conditions/
    ├── stages.go        # Condition stages
    └── stages_test.go
```

```go
// mechanics/go.mod
module github.com/KirkDiggler/rpg-toolkit/mechanics

require (
    github.com/KirkDiggler/rpg-toolkit/core v0.1.0
    github.com/KirkDiggler/rpg-toolkit/pipeline v0.1.0
)
```

### Rulebook Module (Game Implementation)

```
github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e
├── go.mod (depends on core, pipeline, mechanics)
├── refs.go              # All D&D pipeline refs
├── registry.go          # Register all pipelines
├── pipelines/
│   ├── attack.go
│   ├── damage.go
│   └── saves.go
└── stages/
    ├── advantage.go     # D&D specific stages
    └── critical.go
```

```go
// rulebooks/dnd5e/refs.go
package dnd5e

import "github.com/KirkDiggler/rpg-toolkit/core"

var (
    AttackPipelineRef = &pipeline.PipelineRef{
        Ref: core.MustParseString("dnd5e:pipeline:attack"),
    }
    
    DamagePipelineRef = &pipeline.PipelineRef{
        Ref: core.MustParseString("dnd5e:pipeline:damage"),
    }
    
    SavePipelineRef = &pipeline.PipelineRef{
        Ref: core.MustParseString("dnd5e:pipeline:save"),
    }
)
```

## Game Server Integration

```go
// gameserver/server.go
type GameServer struct {
    registry     *pipeline.Registry
    store        DataStore
    continuations map[string][]byte  // Serialized continuations
}

func (s *GameServer) Initialize() {
    // Register D&D pipelines
    dnd5e.RegisterPipelines(s.registry)
    
    // Or Pathfinder
    // pathfinder2e.RegisterPipelines(s.registry)
}

func (s *GameServer) ExecuteAction(action Action) Response {
    // Action includes pipeline ref
    pipeline, err := s.registry.Get(action.PipelineRef)
    if err != nil {
        return ErrorResponse{Error: "unknown pipeline"}
    }
    
    result := pipeline.Process(newContext(), action.Input)
    
    switch r := result.(type) {
    case CompletedResult:
        s.applyData(r.GetData())
        return CompletedResponse{Output: r.Output}
        
    case SuspendedResult:
        // Serialize continuation
        contData := r.GetContinuation()
        contJSON, _ := contData.ToJSON()
        
        contID := uuid.New().String()
        s.continuations[contID] = contJSON
        
        return SuspendedResponse{
            ContinuationID: contID,
            Decision:       r.GetDecision(),
        }
    }
}

func (s *GameServer) ResumeAction(contID string, decision Decision) Response {
    // Load serialized continuation
    contJSON, exists := s.continuations[contID]
    if !exists {
        return ErrorResponse{Error: "continuation not found"}
    }
    
    // Deserialize
    contData, _ := pipeline.LoadContinuation(contJSON)
    
    // Load pipeline by ref
    pipeline, err := s.registry.LoadContinuation(contData)
    if err != nil {
        return ErrorResponse{Error: err}
    }
    
    // Resume
    result := pipeline.Resume(contData, decision)
    
    // Handle result same as ExecuteAction...
}
```

## The Critical Design Decisions

### 1. Core is Minimal
Core ONLY defines interfaces, no implementation:
```go
// core/pipeline/types.go
type Pipeline interface {
    GetRef() *PipelineRef
    Process(ctx *Context, input any) Result
    Resume(cont ContinuationData, decision any) Result
}
```

### 2. Pipeline Module Has Implementation
The `pipeline` module provides the actual execution engine:
```go
// pipeline/executor.go
type Executor struct {
    registry *Registry
}

func (e *Executor) Execute(p Pipeline, input any) Result {
    // Actual execution logic
}
```

### 3. Each Module is Independent
```
core       → Just interfaces
pipeline   → Depends on core
mechanics  → Depends on core, pipeline
dnd5e      → Depends on core, pipeline, mechanics
```

### 4. Refs Enable Serialization
```go
// We know this is the D&D attack pipeline
{
    "pipeline_ref": "dnd5e:pipeline:attack",
    "stage": 3,
    "intermediate": {...}
}
```

## Benefits

1. **Clean Module Boundaries** - Each module has clear dependencies
2. **Serializable Continuations** - Can save/load/resume
3. **Registry Pattern** - Pipelines self-register
4. **Type Safety** - Even with serialization
5. **Game Agnostic** - Core knows nothing about games

## Example: Full Flow

```go
// 1. Client sends action
{
    "type": "action",
    "pipeline_ref": "dnd5e:pipeline:attack",
    "input": {
        "attacker": "fighter-123",
        "target": "goblin-456"
    }
}

// 2. Server loads pipeline by ref
pipeline := registry.Get("dnd5e:pipeline:attack")

// 3. Pipeline suspends for Shield decision
{
    "suspended": true,
    "continuation_id": "abc-123",
    "continuation": {
        "pipeline_ref": "dnd5e:pipeline:attack",
        "stage": 3,
        "intermediate": {...}
    }
}

// 4. Store serialized continuation
redis.Set("cont:abc-123", continuationJSON)

// 5. Later, resume from serialized data
contJSON := redis.Get("cont:abc-123")
pipeline := registry.LoadContinuation(contJSON)
result := pipeline.Resume(contData, decision)
```

The refs make everything work - we can serialize, store, and reconstruct pipelines!