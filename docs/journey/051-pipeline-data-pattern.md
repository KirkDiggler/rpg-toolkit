# Pipeline Data Pattern

**Status: DESIGN EXPLORATION**

*Building on: [Pipeline Suspension and Resumption](pipeline-suspension-resumption.md)*

## The Core Insight

Pipelines don't persist state - they return **Data** that the game server can persist. This keeps pipelines pure and gives the server full control over state management.

## The Data Pattern

```go
// Every pipeline result includes data to persist
type PipelineResult[T any] interface {
    GetOutput() T
    GetData() []Data  // Data for the server to persist
    IsComplete() bool
}

// Data represents a state change
type Data interface {
    GetEntityID() string
    GetOperation() DataOperation
    Apply(store DataStore) error
}

type DataOperation string

const (
    OpUpdate  DataOperation = "update"
    OpAppend  DataOperation = "append"
    OpRemove  DataOperation = "remove"
    OpReplace DataOperation = "replace"
)
```

## Pipeline Results with Data

### Completed Result with Data

```go
type CompletedResult[T any] struct {
    Output T
    Data   []Data  // State changes to persist
    Events []Event // Events to publish
}

// Example: Attack pipeline returns damage data
attackResult := CompletedResult[AttackOutput]{
    Output: AttackOutput{
        Hit:    true,
        Damage: 15,
    },
    Data: []Data{
        &HPData{
            EntityID:  "goblin-123",
            Operation: OpUpdate,
            HP:        -15,  // Relative change
        },
        &CombatLogData{
            Operation: OpAppend,
            Entry:     "Fighter hits Goblin for 15 damage",
        },
    },
}
```

### Suspended Result with Data

```go
type SuspendedResult[T any] struct {
    Continuation ContinuationData  // Just data, not functions!
    Decision     DecisionRequest
    Data         []Data           // Partial state changes
}

// ContinuationData is pure data (no functions)
type ContinuationData struct {
    PipelineID   string                 `json:"pipeline_id"`
    PipelineType string                 `json:"pipeline_type"`
    Stage        int                    `json:"stage"`
    Input        json.RawMessage        `json:"input"`
    Intermediate json.RawMessage        `json:"intermediate"`
    Context      ContextData            `json:"context"`
    Metadata     map[string]interface{} `json:"metadata"`
}
```

## Example: Damage Pipeline with Concentration

```go
func (d *DamagePipeline) Process(ctx *PipelineContext, input DamageInput) PipelineResult[DamageOutput] {
    // Calculate damage
    finalDamage := d.calculateDamage(input)
    
    // Prepare data for persistence
    dataToReturn := []Data{
        &HPData{
            EntityID:  input.Target,
            Operation: OpUpdate,
            HP:        -finalDamage,
        },
    }
    
    // Check if concentration needed (rulebook knows this!)
    if d.rulebook.RequiresConcentrationCheck(input.Target) {
        concInput := ConcentrationInput{
            Entity: input.Target,
            Damage: finalDamage,
        }
        
        // Run concentration pipeline
        concResult := d.concentrationPipeline.Process(ctx.Nest("concentration"), concInput)
        
        switch r := concResult.(type) {
        case CompletedResult[ConcentrationOutput]:
            // Concentration check completed
            if !r.Output.Maintained {
                dataToReturn = append(dataToReturn, &ConditionData{
                    EntityID:  input.Target,
                    Operation: OpRemove,
                    Condition: "concentrating",
                })
                dataToReturn = append(dataToReturn, &SpellData{
                    EntityID:  input.Target,
                    Operation: OpRemove,
                    SpellID:   r.Output.SpellID,
                })
            }
            
            // Add concentration data to our return
            dataToReturn = append(dataToReturn, r.Data...)
            
            return CompletedResult[DamageOutput]{
                Output: DamageOutput{
                    FinalDamage:             finalDamage,
                    ConcentrationMaintained: r.Output.Maintained,
                },
                Data: dataToReturn,
            }
            
        case SuspendedResult[ConcentrationOutput]:
            // Need decision for concentration save
            return SuspendedResult[DamageOutput]{
                Continuation: ContinuationData{
                    PipelineID:   d.ID,
                    PipelineType: "DamagePipeline",
                    Stage:        StageConcentration,
                    Input:        marshal(input),
                    Intermediate: marshal(DamageIntermediate{
                        FinalDamage:    finalDamage,
                        DataSoFar:      dataToReturn,
                        ConcentrationContinuation: r.Continuation,
                    }),
                    Context: ctx.ToData(),
                },
                Decision: r.Decision,
                Data:     dataToReturn, // Partial data (damage applied, concentration pending)
            }
        }
    }
    
    return CompletedResult[DamageOutput]{
        Output: DamageOutput{FinalDamage: finalDamage},
        Data:   dataToReturn,
    }
}
```

## The Game Server's Role

```go
type GameServer struct {
    store        DataStore
    pipelines    map[string]Pipeline
    continuations map[string]ContinuationData
}

func (s *GameServer) ExecuteAction(action Action) Response {
    pipeline := s.pipelines[action.PipelineType]
    result := pipeline.Process(newContext(), action.Input)
    
    switch r := result.(type) {
    case CompletedResult[any]:
        // Apply all data changes
        for _, data := range r.Data {
            if err := data.Apply(s.store); err != nil {
                return ErrorResponse{Error: err}
            }
        }
        
        // Publish events
        for _, event := range r.Events {
            s.eventBus.Publish(event)
        }
        
        return CompletedResponse{Output: r.Output}
        
    case SuspendedResult[any]:
        // Apply partial data (e.g., damage even though concentration pending)
        for _, data := range r.Data {
            if err := data.Apply(s.store); err != nil {
                return ErrorResponse{Error: err}
            }
        }
        
        // Store continuation DATA (not functions!)
        continuationID := uuid.New().String()
        s.continuations[continuationID] = r.Continuation
        
        return SuspendedResponse{
            ContinuationID: continuationID,
            Decision:       r.Decision,
        }
    }
}

func (s *GameServer) ResumeAction(continuationID string, decision Decision) Response {
    // Retrieve continuation data
    contData, exists := s.continuations[continuationID]
    if !exists {
        return ErrorResponse{Error: "continuation not found"}
    }
    
    // Reconstruct pipeline
    pipeline := s.pipelines[contData.PipelineType]
    
    // Resume with data and decision
    result := pipeline.Resume(contData, decision)
    
    // Same handling as ExecuteAction...
    switch r := result.(type) {
    case CompletedResult[any]:
        // Apply data, delete continuation
        for _, data := range r.Data {
            data.Apply(s.store)
        }
        delete(s.continuations, continuationID)
        return CompletedResponse{Output: r.Output}
        
    case SuspendedResult[any]:
        // Update continuation, apply partial data
        s.continuations[continuationID] = r.Continuation
        for _, data := range r.Data {
            data.Apply(s.store)
        }
        return SuspendedResponse{
            ContinuationID: continuationID,
            Decision:       r.Decision,
        }
    }
}
```

## Specific Data Types

```go
// HP changes
type HPData struct {
    EntityID  string
    Operation DataOperation
    HP        int  // Relative if Update, absolute if Replace
    Temporary bool // Temporary HP
}

// Condition changes
type ConditionData struct {
    EntityID  string
    Operation DataOperation  
    Condition string
    Duration  *int
    Source    string
}

// Resource changes (spell slots, etc)
type ResourceData struct {
    EntityID     string
    Operation    DataOperation
    ResourceType string
    Amount       int
    Level        *int  // For spell slots
}

// Combat log entries
type CombatLogData struct {
    Operation DataOperation
    Entry     string
    Timestamp time.Time
    Round     int
    Turn      string
}

// Death saves
type DeathSaveData struct {
    EntityID  string
    Operation DataOperation
    Successes int
    Failures  int
}
```

## Pipeline Resume Pattern

```go
// Pipelines implement Resume to continue from data
type ResumablePipeline[TInput, TOutput any] interface {
    Pipeline[TInput, TOutput]
    Resume(continuation ContinuationData, decision Decision) PipelineResult[TOutput]
}

func (p *AttackPipeline) Resume(cont ContinuationData, decision Decision) PipelineResult[AttackOutput] {
    // Unmarshal saved state
    var intermediate AttackIntermediate
    json.Unmarshal(cont.Intermediate, &intermediate)
    
    // Apply decision
    if decision.Choice == "cast_shield" {
        intermediate.TargetAC += 5
        // Add data for resource consumption
        intermediate.DataToReturn = append(intermediate.DataToReturn, &ResourceData{
            EntityID:     decision.EntityID,
            Operation:    OpUpdate,
            ResourceType: "spell_slot",
            Level:        1,
            Amount:       -1,
        })
    }
    
    // Continue from saved stage
    for i := cont.Stage + 1; i < len(p.stages); i++ {
        stageResult := p.stages[i].Process(intermediate)
        intermediate = stageResult.Output
        intermediate.DataToReturn = append(intermediate.DataToReturn, stageResult.Data...)
    }
    
    // Return completed result with all data
    return CompletedResult[AttackOutput]{
        Output: p.convertToOutput(intermediate),
        Data:   intermediate.DataToReturn,
    }
}
```

## Key Benefits of Data Pattern

### 1. Pure Pipelines
Pipelines don't touch storage - they just return data:
```go
// Pipeline doesn't know about storage implementation
return CompletedResult[DamageOutput]{
    Output: output,
    Data: []Data{
        &HPData{EntityID: target, Operation: OpUpdate, HP: -damage},
    },
}
```

### 2. Server Control
The server decides how to apply data:
```go
// Server can validate, batch, transaction wrap, etc.
func (s *GameServer) applyData(data []Data) error {
    tx := s.db.Begin()
    defer tx.Rollback()
    
    for _, d := range data {
        if err := s.validate(d); err != nil {
            return err
        }
        if err := d.Apply(tx); err != nil {
            return err
        }
    }
    
    return tx.Commit()
}
```

### 3. Serializable Continuations
No functions in continuations, just data:
```go
// Can be stored in Redis, database, sent to client
type ContinuationData struct {
    PipelineType string          `json:"pipeline_type"`
    Stage        int             `json:"stage"`
    Input        json.RawMessage `json:"input"`
    // No function pointers!
}
```

### 4. Audit Trail
Every change is explicit data:
```go
// Can log/audit every data operation
for _, data := range result.Data {
    log.Printf("Operation: %s on %s: %+v", 
        data.GetOperation(), 
        data.GetEntityID(), 
        data)
}
```

### 5. Testability
```go
func TestDamagePipeline(t *testing.T) {
    result := pipeline.Process(ctx, DamageInput{
        Target: "goblin",
        Amount: 10,
    })
    
    // Just check returned data, don't need mock store
    assert.Contains(t, result.Data, &HPData{
        EntityID:  "goblin",
        Operation: OpUpdate,
        HP:        -10,
    })
}
```

## The Complete Flow

```go
// 1. Client requests action
Action{Type: "attack", Attacker: "fighter", Target: "wizard"}
    ↓
// 2. Pipeline processes, returns data
CompletedResult{
    Output: AttackOutput{Hit: true, Damage: 8},
    Data: [
        HPData{EntityID: "wizard", HP: -8},
        CombatLogData{Entry: "Fighter hits Wizard for 8"},
    ],
}
    ↓
// 3. Server applies data
store.UpdateHP("wizard", -8)
store.AppendLog("Fighter hits Wizard for 8")
    ↓
// 4. Server returns response
Response{Success: true, Output: AttackOutput{...}}
```

Or with suspension:

```go
// 1. Pipeline needs decision
SuspendedResult{
    Continuation: ContinuationData{...},  // Pure data
    Decision: DecisionRequest{...},
    Data: [HPData{...}],  // Partial data applied
}
    ↓
// 2. Server stores continuation data
continuations["abc-123"] = ContinuationData{...}
    ↓
// 3. Client makes decision
Decision{Choice: "cast_shield"}
    ↓
// 4. Pipeline resumes from data
pipeline.Resume(continuationData, decision)
    ↓
// 5. Returns more data
CompletedResult{
    Data: [
        ResourceData{SpellSlot: -1},
        CombatLogData{Entry: "Wizard casts Shield"},
    ],
}
```

## The Beautiful Unification

The Data pattern unifies everything:
- **State changes** are Data
- **Continuations** are Data  
- **Context** is Data
- **Decisions** are Data

Pipelines are just pure functions that transform:
```
(Input + Context + Maybe Decision) → (Output + Data + Maybe Continuation)
```

The toolkit provides the pipeline machinery. The rulebooks define what data to return. The game server controls persistence.