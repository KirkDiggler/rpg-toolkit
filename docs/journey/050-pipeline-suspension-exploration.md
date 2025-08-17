# Pipeline Suspension and Resumption

**Status: DESIGN EXPLORATION**

*Extension of: [Pipeline Execution Patterns](pipeline-execution-patterns.md)*

## The Core Problem

When a pipeline needs a player decision (casting Shield, using Legendary Resistance, choosing Counterspell level), we need to:
1. **Suspend** execution at that exact point
2. **Serialize** the pipeline state 
3. **Return** to the client with decision request
4. **Resume** when the decision arrives
5. **Continue** from exactly where we left off

## Solution: Continuation-Based Pipelines

### Pipeline Execution Result Types

```go
type PipelineResult interface {
    isComplete() bool
}

// Normal completion
type CompletedResult[T any] struct {
    Output T
    Error  error
}

func (CompletedResult[T]) isComplete() bool { return true }

// Needs player decision
type SuspendedResult struct {
    Continuation *PipelineContinuation
    Decision     DecisionRequest
}

func (SuspendedResult) isComplete() bool { return false }
```

### The Continuation Object

```go
type PipelineContinuation struct {
    // Pipeline identification
    PipelineID   string
    PipelineType string
    
    // Execution state
    Stage        int                 // Which stage we're at
    StageState   map[string]any      // Stage-specific state
    Input        any                 // Original input
    Intermediate any                 // Result so far
    Context      *PipelineContext    // Full context including call stack
    
    // Resumption info
    WaitingFor   DecisionType
    ResumeFn     func(decision Decision) PipelineResult
}

// Serializable for storage
func (c *PipelineContinuation) Serialize() ([]byte, error) {
    return json.Marshal(c)
}

func DeserializeContinuation(data []byte) (*PipelineContinuation, error) {
    var c PipelineContinuation
    err := json.Unmarshal(data, &c)
    return &c, err
}
```

### Decision Request Structure

```go
type DecisionRequest struct {
    Type        DecisionType
    EntityID    string
    Options     []DecisionOption
    Context     DecisionContext  // What the player needs to know
    TimeLimit   *time.Duration   // Optional timeout
}

type DecisionType string

const (
    DecisionReaction      DecisionType = "reaction"       // Use reaction?
    DecisionSpellLevel    DecisionType = "spell_level"    // What level?
    DecisionTarget        DecisionType = "target"         // Which target?
    DecisionLegendary     DecisionType = "legendary"      // Use legendary resistance?
    DecisionOpportunity   DecisionType = "opportunity"    // Take opportunity attack?
)

type DecisionOption struct {
    ID          string
    Label       string
    Cost        *Cost  // Spell slot, reaction, etc.
    Available   bool
    Requirement string // Why it might not be available
}
```

## Example: Shield Reaction with Suspension

### The Interruptible Attack Pipeline

```go
func (p *AttackPipeline) Process(ctx *PipelineContext, input AttackInput) PipelineResult {
    // Roll attack
    stages := p.getStages()
    result := input
    
    for i, stage := range stages {
        result = stage.Process(ctx, result)
        
        // After roll, check for Shield reaction
        if i == p.afterRollStageIndex {
            if shield := p.checkShieldReaction(ctx, result); shield != nil {
                // Need player decision!
                return SuspendedResult{
                    Continuation: &PipelineContinuation{
                        PipelineID:   p.ID,
                        PipelineType: "AttackPipeline",
                        Stage:        i,
                        Input:        input,
                        Intermediate: result,
                        Context:      ctx.Clone(),
                        WaitingFor:   DecisionReaction,
                        ResumeFn:     p.createResumeFunc(i, result),
                    },
                    Decision: DecisionRequest{
                        Type:     DecisionReaction,
                        EntityID: shield.CasterID,
                        Options: []DecisionOption{
                            {
                                ID:    "cast_shield",
                                Label: "Cast Shield (+5 AC)",
                                Cost:  &Cost{SpellSlot: 1, Level: 1},
                                Available: shield.HasSlot,
                            },
                            {
                                ID:    "no_reaction",
                                Label: "Don't use reaction",
                                Available: true,
                            },
                        },
                        Context: DecisionContext{
                            "incoming_attack": result.Total,
                            "current_ac":      result.TargetAC,
                            "would_hit":       result.Total >= result.TargetAC,
                        },
                    },
                }
            }
        }
    }
    
    // Normal completion
    return CompletedResult[AttackOutput]{
        Output: AttackOutput{
            Hit:    result.Total >= result.TargetAC,
            Damage: result.Damage,
        },
    }
}

// Resume function creation
func (p *AttackPipeline) createResumeFunc(stageIndex int, intermediate AttackIntermediate) func(Decision) PipelineResult {
    return func(decision Decision) PipelineResult {
        // Apply decision
        if decision.Choice == "cast_shield" {
            intermediate.TargetAC += 5
            intermediate.ReactionsUsed = append(intermediate.ReactionsUsed, "shield")
        }
        
        // Continue from next stage
        for i := stageIndex + 1; i < len(p.stages); i++ {
            intermediate = p.stages[i].Process(p.savedContext, intermediate)
        }
        
        return CompletedResult[AttackOutput]{
            Output: convertToOutput(intermediate),
        }
    }
}
```

## The Execution Loop

### Server-Side Game Loop

```go
type GameEngine struct {
    suspended map[string]*PipelineContinuation
    events    EventBus
}

func (e *GameEngine) ExecuteAction(ctx context.Context, action Action) ActionResult {
    pipeline := action.GetPipeline()
    result := pipeline.Process(ctx, action.GetInput())
    
    switch r := result.(type) {
    case CompletedResult[any]:
        // Action completed immediately
        return ActionResult{Completed: true, Output: r.Output}
        
    case SuspendedResult:
        // Save continuation
        continuationID := uuid.New().String()
        e.suspended[continuationID] = r.Continuation
        
        // Return decision request to client
        return ActionResult{
            Suspended:      true,
            ContinuationID: continuationID,
            Decision:       r.Decision,
        }
    }
}

func (e *GameEngine) ResumeAction(continuationID string, decision Decision) ActionResult {
    continuation, exists := e.suspended[continuationID]
    if !exists {
        return ActionResult{Error: "continuation not found"}
    }
    
    // Resume pipeline
    result := continuation.ResumeFn(decision)
    
    // Handle result (might suspend again!)
    switch r := result.(type) {
    case CompletedResult[any]:
        delete(e.suspended, continuationID)
        return ActionResult{Completed: true, Output: r.Output}
        
    case SuspendedResult:
        // Update continuation
        e.suspended[continuationID] = r.Continuation
        return ActionResult{
            Suspended:      true,
            ContinuationID: continuationID,
            Decision:       r.Decision,
        }
    }
}
```

## Pipeline-Specific Decision Logic

### Damage Pipeline Knows About Concentration

```go
func (d *DamagePipeline) Process(ctx *PipelineContext, input DamageInput) PipelineResult {
    // Calculate damage
    damage := d.calculateDamage(input)
    
    // Apply damage
    input.Target.TakeDamage(damage)
    
    // The damage pipeline KNOWS it needs to check concentration
    // This is rulebook-specific logic!
    if d.rulebook.RequiresConcentrationCheck(input.Target) {
        concInput := ConcentrationInput{
            Entity: input.Target,
            Damage: damage,
        }
        
        // Trigger concentration pipeline
        concResult := d.concentrationPipeline.Process(ctx.Nest("concentration"), concInput)
        
        // Concentration might suspend for save decisions
        if suspended, ok := concResult.(SuspendedResult); ok {
            // Wrap our continuation
            return SuspendedResult{
                Continuation: &PipelineContinuation{
                    // ... damage pipeline state
                    ResumeFn: func(decision Decision) PipelineResult {
                        // Resume concentration pipeline
                        concResult = suspended.Continuation.ResumeFn(decision)
                        
                        // Then complete damage pipeline
                        return d.completeDamage(damage, concResult)
                    },
                },
                Decision: suspended.Decision,
            }
        }
        
        // Concentration completed immediately
        return d.completeDamage(damage, concResult)
    }
    
    return CompletedResult[DamageOutput]{
        Output: DamageOutput{
            FinalDamage: damage,
        },
    }
}
```

## Complex Example: Nested Suspensions

### Counterspell Chain with Multiple Decisions

```go
// Wizard A casts Fireball
SpellCastPipeline.Process(ctx, SpellCastInput{Caster: wizardA, Spell: "fireball"})
    ↓
    SUSPEND: "Wizard B, do you want to Counterspell?"
    ↓
    Decision: YES (at 3rd level)
    ↓
    Resume → CounterspellPipeline.Process(...)
        ↓
        SUSPEND: "Wizard A, do you want to Counter-Counterspell?"
        ↓
        Decision: YES (at 4th level)
        ↓
        Resume → CounterspellPipeline.Process(...)
            ↓
            Complete: Counter-Counterspell succeeds
        ↓
        Resume → Counterspell fails (was countered)
    ↓
    Resume → Fireball proceeds
    ↓
    Complete: Fireball cast successfully
```

### The Continuation Stack

```go
type ContinuationStack struct {
    continuations []*PipelineContinuation
}

// When we suspend nested pipelines:
suspension1 := &PipelineContinuation{
    PipelineType: "SpellCastPipeline",
    WaitingFor:   DecisionReaction, // Counterspell?
    ResumeFn: func(d1 Decision) PipelineResult {
        if d1.Choice == "counterspell" {
            // Start counterspell pipeline
            result := CounterspellPipeline.Process(...)
            
            if suspended, ok := result.(SuspendedResult); ok {
                // Nested suspension!
                return SuspendedResult{
                    Continuation: &PipelineContinuation{
                        ResumeFn: func(d2 Decision) PipelineResult {
                            // Resume counterspell
                            counterResult := suspended.Continuation.ResumeFn(d2)
                            // Then resume spell cast
                            return continueSpellCast(counterResult)
                        },
                    },
                    Decision: suspended.Decision,
                }
            }
        }
        return continueSpellCast(d1)
    },
}
```

## State Persistence Options

### Option 1: In-Memory (Simple)
```go
type InMemoryStore struct {
    continuations map[string]*PipelineContinuation
    mu            sync.RWMutex
}
```

### Option 2: Redis (Distributed)
```go
type RedisStore struct {
    client *redis.Client
    ttl    time.Duration // Auto-expire old continuations
}

func (s *RedisStore) Save(id string, cont *PipelineContinuation) error {
    data, _ := cont.Serialize()
    return s.client.Set(ctx, "continuation:"+id, data, s.ttl).Err()
}
```

### Option 3: Database (Persistent)
```go
type DBStore struct {
    db *sql.DB
}

func (s *DBStore) Save(id string, cont *PipelineContinuation) error {
    data, _ := cont.Serialize()
    _, err := s.db.Exec(`
        INSERT INTO pipeline_continuations (id, data, created_at)
        VALUES (?, ?, ?)
    `, id, data, time.Now())
    return err
}
```

## Client Integration

### Request/Response Flow

```typescript
// Client sends action
const response = await gameAPI.executeAction({
    type: "attack",
    attacker: "fighter-123",
    target: "goblin-456"
});

if (response.suspended) {
    // Show decision UI
    const decision = await showDecisionModal(response.decision);
    
    // Resume with decision
    const resumed = await gameAPI.resumeAction(
        response.continuationId,
        decision
    );
    
    // Might need more decisions!
    while (resumed.suspended) {
        const decision = await showDecisionModal(resumed.decision);
        resumed = await gameAPI.resumeAction(
            resumed.continuationId,
            decision
        );
    }
    
    // Finally complete
    handleActionResult(resumed.output);
} else {
    // Completed immediately
    handleActionResult(response.output);
}
```

## Key Design Principles

### 1. Pipelines Own Their Suspension Logic
Each pipeline knows when it needs decisions:
- AttackPipeline knows about Shield timing
- DamagePipeline knows about concentration
- SpellCastPipeline knows about Counterspell

### 2. Continuations are Self-Contained
Everything needed to resume is in the continuation:
- Pipeline state
- Execution context
- Resume function
- Original input

### 3. Decisions are Explicit
Not just "yes/no" but rich decision objects:
- What options are available
- What costs are involved
- What context the player needs

### 4. Nesting is Natural
Suspended pipelines can trigger other pipelines that also suspend:
- The continuation stack handles this
- Each resume function knows its context

## Benefits

1. **True Async Support**: Can handle network latency, player thinking time
2. **Save/Load Games**: Continuations can be persisted
3. **Replay/Undo**: Can restart from any continuation point
4. **Debugging**: Can inspect suspended state
5. **AI Integration**: AI can analyze decision points
6. **Multiplayer**: Different players can have pending decisions

## The Beautiful Part

The rulebook defines what checks are needed:

```go
// D&D 5e Damage Pipeline
func (d *DnD5eDamagePipeline) Process(...) {
    // D&D knows about concentration
    if target.IsConcentrating() {
        // Trigger concentration check
    }
}

// Pathfinder 2e Damage Pipeline  
func (p *PF2eDamagePipeline) Process(...) {
    // Pathfinder has different rules!
    if target.HasPersistentDamage() {
        // Different check
    }
}

// The toolkit just provides the suspension mechanism!
```