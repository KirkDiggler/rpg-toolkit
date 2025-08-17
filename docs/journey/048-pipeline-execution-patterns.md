# Journey 048: Pipeline Execution Patterns

*Date: August 2024*
*Context: Exploring nested, interrupt, and sequential execution patterns*
*Outcome: Identified three core patterns that work together*

**Status: DESIGN EXPLORATION**

*Building on: [ADR-0025 Pipeline Architecture](../adr/0025-pipeline-architecture.md)*

## Core Question

How do pipelines handle nested execution, interrupts, and general processing in a way that's both powerful and understandable?

## The Three Execution Patterns

### 1. Sequential Processing (Simple)

The base case - stages execute in order, each transforming the value:

```go
// Input flows through stages sequentially
AttackPipeline:
    BaseRoll    → d20 + 3 = 18
    Proficiency → 18 + 2 = 20  
    Features    → 20 + 2 = 22 (rage)
    Spells      → 22 + 1d4 = 24 (bless)
    Result      → Hit with 24
```

This is straightforward and covers 80% of use cases.

### 2. Nested Processing (Cascading)

Pipelines triggering other pipelines, creating a tree of execution:

```go
// Attack triggers damage which triggers concentration
AttackPipeline.Process()
    ├─ DamagePipeline.Process()
    │   ├─ ResistancePipeline.Process()
    │   └─ ConcentrationPipeline.Process()
    │       └─ SavePipeline.Process()
    │           └─ ModifierPipeline.Process()
    └─ return AttackOutput{hit: true, damageDealt: 7}
```

#### Key Design Decisions for Nesting

**Context Inheritance**
```go
type PipelineContext struct {
    // Parent context for traversal
    Parent *PipelineContext
    
    // Track nesting depth (prevent infinite recursion)
    Depth int
    MaxDepth int // default: 10
    
    // Execution trace for debugging
    CallStack []string
    
    // Shared state flows down
    Round int
    CurrentTurn EntityID
    
    // Results bubble up
    Results map[string]any
}

func (ctx *PipelineContext) Nest(name string) *PipelineContext {
    if ctx.Depth >= ctx.MaxDepth {
        panic("pipeline recursion limit exceeded")
    }
    
    return &PipelineContext{
        Parent: ctx,
        Depth: ctx.Depth + 1,
        MaxDepth: ctx.MaxDepth,
        CallStack: append(ctx.CallStack, name),
        Round: ctx.Round,
        CurrentTurn: ctx.CurrentTurn,
        Results: make(map[string]any),
    }
}
```

**Triggering Nested Pipelines**
```go
func (d *DamagePipeline) Process(ctx *PipelineContext, input DamageInput) (DamageOutput, error) {
    output := d.processStages(ctx, input)
    
    // Conditionally trigger nested pipeline
    if target.IsConcentrating() {
        // Create nested context
        nestedCtx := ctx.Nest("concentration")
        
        // Execute nested pipeline
        concResult, err := ConcentrationPipeline.Process(nestedCtx, ConcentrationInput{
            Caster: input.Target,
            Damage: output.FinalDamage,
        })
        
        // Store result in parent context
        ctx.Results["concentration"] = concResult
        
        // React to nested result
        if concResult.Broken {
            output.AdditionalEffects = append(output.AdditionalEffects, "Concentration broken")
        }
    }
    
    return output, nil
}
```

### 3. Interrupt Processing (Reactions)

The complex case - execution can be interrupted, modified, or cancelled:

```go
// Counterspell interrupting a spell cast
SpellCastPipeline.Process()
    ├─ CheckInterrupt("before-cast")
    │   └─ CounterspellPipeline.Process() // Reaction!
    │       ├─ SavePipeline.Process()
    │       └─ return Cancel: true
    └─ return SpellOutput{cancelled: true}
```

#### Interrupt Design Pattern

**Interrupt Points**
```go
type InterruptPoint string

const (
    BeforeRoll     InterruptPoint = "before-roll"
    AfterRoll      InterruptPoint = "after-roll"
    BeforeDamage   InterruptPoint = "before-damage"
    AfterDamage    InterruptPoint = "after-damage"
    BeforeMove     InterruptPoint = "before-move"
    AfterMove      InterruptPoint = "after-move"
)

type InterruptiblePipeline interface {
    Pipeline
    GetInterruptPoints() []InterruptPoint
}
```

**Gathering and Processing Interrupts**
```go
type Interrupt struct {
    Source    EntityID
    Priority  int // Higher priority goes first
    Pipeline  Pipeline
    Input     any
    Condition func(ctx *PipelineContext) bool
}

func (p *AttackPipeline) Process(ctx *PipelineContext, input AttackInput) (AttackOutput, error) {
    // Check for interrupts before roll
    if interrupts := p.gatherInterrupts(ctx, BeforeRoll); len(interrupts) > 0 {
        for _, interrupt := range p.sortByPriority(interrupts) {
            if !interrupt.Condition(ctx) {
                continue
            }
            
            result := interrupt.Pipeline.Process(ctx.Nest("interrupt"), interrupt.Input)
            
            // Handle different interrupt outcomes
            switch r := result.(type) {
            case CancelResult:
                return AttackOutput{Cancelled: true}, nil
            case ModifyResult:
                input = r.ModifyInput(input).(AttackInput)
            case ReplaceResult:
                return r.Output.(AttackOutput), nil
            }
        }
    }
    
    // Continue with normal processing
    output := p.processStages(ctx, input)
    
    // Check for interrupts after roll
    if interrupts := p.gatherInterrupts(ctx, AfterRoll); len(interrupts) > 0 {
        // Process post-roll interrupts...
    }
    
    return output, nil
}
```

## Execution Flow Control

### Pipeline Results Can Control Flow

```go
type PipelineResult interface {
    // Marker interface for pipeline results
}

type ContinueResult struct {
    Value any
}

type CancelResult struct {
    Reason string
}

type ReplaceResult struct {
    Output any
}

type TriggerResult struct {
    Pipeline Pipeline
    Input    any
}
```

### Example: Shield Spell Reaction

```go
// Attack is rolled, triggers Shield reaction
func (s *ShieldReactionPipeline) Process(ctx *PipelineContext, input ShieldInput) (PipelineResult, error) {
    // Player decides whether to cast Shield
    if !player.ChoosesToCastShield(input.IncomingAttack) {
        return ContinueResult{}, nil
    }
    
    // Shield adds +5 AC
    return ModifyResult{
        Modify: func(attack any) any {
            a := attack.(AttackInput)
            a.TargetAC += 5
            return a
        },
    }, nil
}
```

## Assumption Verification

### Assumption 1: "Nested pipelines share context but maintain isolation"

**Verification:**
```go
func TestNestedContextIsolation(t *testing.T) {
    parentCtx := NewPipelineContext()
    parentCtx.Set("value", 10)
    
    nestedCtx := parentCtx.Nest("child")
    nestedCtx.Set("value", 20)
    
    // Parent unchanged
    assert.Equal(t, 10, parentCtx.Get("value"))
    // Child has its own value
    assert.Equal(t, 20, nestedCtx.Get("value"))
    // Child can access parent
    assert.Equal(t, parentCtx, nestedCtx.Parent)
}
```
✅ **Verified**: Nested contexts maintain isolation while allowing traversal

### Assumption 2: "Interrupts can meaningfully modify pipeline execution"

**Verification:**
```go
func TestInterruptModification(t *testing.T) {
    pipeline := NewAttackPipeline()
    ctx := NewPipelineContext()
    
    // Register Shield interrupt
    ctx.RegisterInterrupt(AfterRoll, Interrupt{
        Priority: 100,
        Pipeline: ShieldPipeline,
        Condition: func(ctx *PipelineContext) bool {
            return ctx.Get("roll").(int) >= 15 // Only if attack might hit
        },
    })
    
    result := pipeline.Process(ctx, AttackInput{Roll: 18, TargetAC: 20})
    
    // Shield should have triggered, making attack miss
    assert.False(t, result.Hit)
}
```
✅ **Verified**: Interrupts can inspect state and modify execution

### Assumption 3: "Deep nesting is detectable and preventable"

**Verification:**
```go
func TestRecursionPrevention(t *testing.T) {
    pipeline := NewRecursivePipeline() // Calls itself
    ctx := NewPipelineContext()
    ctx.MaxDepth = 5
    
    assert.Panics(t, func() {
        pipeline.Process(ctx, nil)
    })
    
    // Call stack should show the recursion
    assert.Len(t, ctx.CallStack, 5)
}
```
✅ **Verified**: Recursion limits prevent infinite loops

## Design Principles

### 1. Explicit Over Implicit
- Nested pipelines are explicitly triggered, not automatic
- Interrupt points are explicitly defined
- Context nesting is explicit via `ctx.Nest()`

### 2. Traceable Execution
- Full call stack available in context
- Each pipeline logs its entry/exit
- Results stored in context for debugging

### 3. Predictable Ordering
- Stages execute in defined order
- Interrupts sorted by priority
- Nested pipelines complete before parent continues

### 4. Type Safety Throughout
- Each pipeline has typed input/output
- Interrupt results are typed
- Context values are typed (with generics)

## Implementation Priority

### Phase 1: Sequential (Simple)
- Basic pipeline interface
- Stage composition
- Context threading
- Simple transformations

### Phase 2: Nested (Essential) 
- Context nesting
- Result bubbling
- Recursion prevention
- Call stack tracking

### Phase 3: Interrupts (Complex)
- Interrupt points
- Priority system
- Conditional execution
- Flow modification

## Open Questions

1. **Should pipelines be stateless or stateful?**
   - Stateless is simpler and more testable
   - Stateful might be needed for complex mechanics
   
2. **How do we handle partial failures in nested pipelines?**
   - Continue with degraded result?
   - Rollback entire operation?
   - Let parent decide?

3. **Should interrupt registration be static or dynamic?**
   - Static: Defined at pipeline creation
   - Dynamic: Can change during game

4. **How do we handle async operations?**
   - Player decisions
   - Network calls
   - Animations

## Example: Complete Attack with All Patterns

```go
// Demonstrates all three patterns in one flow
AttackAction
    ├─ [INTERRUPT] Check for Defensive Reactions
    │   └─ ShieldPipeline (Priority: 100)
    ├─ [SEQUENTIAL] AttackPipeline
    │   ├─ BaseRollStage
    │   ├─ ProficiencyStage
    │   └─ ModifierStage
    ├─ [INTERRUPT] Check for Offensive Reactions
    │   └─ RipostePipeline (Priority: 50)
    ├─ [NESTED] DamagePipeline (if hit)
    │   ├─ [SEQUENTIAL] CalculateDamageStages
    │   ├─ [NESTED] ResistancePipeline
    │   └─ [NESTED] ConcentrationPipeline
    │       └─ [NESTED] SavePipeline
    └─ Return: Complete attack result with all effects
```

## Conclusion

The three patterns - Sequential, Nested, and Interrupt - provide a complete execution model:

1. **Sequential** handles simple transformations (90% of cases)
2. **Nested** handles cascading effects naturally
3. **Interrupt** handles reactions and complex timing

Together they can express any game mechanic while maintaining type safety, testability, and debuggability.

The key insight: **Make the complex cases explicit rather than hiding them in event handlers.**