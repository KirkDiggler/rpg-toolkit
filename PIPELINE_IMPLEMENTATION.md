# Pipeline Implementation Issue

## Goal
Implement the pipeline system to enable the combat room demo where players can attack a combat dummy using clean, testable pipelines instead of complex event handlers.

## Scope (Keep It Simple)
Start with the minimal pipeline infrastructure needed for a basic attack flow. No premature abstraction.

## Implementation Tasks

### Phase 1: Core Pipeline Infrastructure
Create `pipeline` module with:

1. **Basic Types** (`pipeline/types.go`)
   - `Pipeline` interface with `Process(ctx *Context, input any) Result`
   - `Result` interface with `IsComplete()` and `GetData()`
   - `Context` struct with `Round`, `CurrentTurn`, `Registry`
   - `Data` interface for state changes

2. **Simple Executor** (`pipeline/executor.go`)
   - `Sequential()` helper to chain stages
   - Basic stage execution

3. **Registry** (`pipeline/registry.go`)
   - `Register(ref *core.Ref, factory PipelineFactory)`
   - `Get(ref *core.Ref) Pipeline`

### Phase 2: D&D Combat Demo
Create minimal D&D rulebook implementation:

1. **Attack Pipeline** (`rulebooks/dnd5e/attack.go`)
   - D20 roll + modifier
   - Compare to AC
   - Calculate damage if hit
   - Return HP data change

2. **Pipeline Registration** (`rulebooks/dnd5e/registry.go`)
   - Register attack pipeline with ref `"dnd5e:pipeline:attack"`

### Phase 3: Combat Room Demo
Wire it up in a simple demo:

1. **Simple Server**
   - Create encounter with combat dummy
   - Execute attack pipeline
   - Apply returned data (HP changes)
   - Return results

2. **Basic Test**
   ```go
   func TestCombatRoomAttack(t *testing.T) {
       registry := pipeline.NewRegistry()
       dnd5e.RegisterPipelines(registry)
       
       result := registry.Get(dnd5e.AttackRef).Process(ctx, AttackInput{
           Attacker: "player",
           Target: "dummy",
           Modifier: 5,
           TargetAC: 10,
       })
       
       // Check we got HP data back
       assert.NotEmpty(t, result.GetData())
   }
   ```

## Success Criteria
- [ ] Attack pipeline executes and returns data
- [ ] No side effects in pipeline (pure function)
- [ ] Data pattern works (pipeline returns changes, server applies them)
- [ ] Simpler than previous event-based implementation

## Out of Scope (For Now)
- Suspension/resumption (add later)
- Complex stages (advantage, critical, etc.)
- Multiple pipelines (damage, saves, etc.)
- gRPC streaming (add after basic pipeline works)

## Why This Matters
The combat room demo was extremely complicated with events. This pipeline approach should make it trivial:
```go
// Old way: Events triggering events, state mutations everywhere
// New way: Just this
result := attackPipeline.Process(ctx, input)
applyData(result.GetData())
```

## Code Structure
```
pipeline/
├── types.go      # Interfaces
├── executor.go   # Sequential execution
├── registry.go   # Pipeline registry
└── data.go       # Data types (HP, Log, etc.)

rulebooks/dnd5e/
├── refs.go       # Pipeline refs
├── attack.go     # Attack pipeline
└── registry.go   # Registration
```

## Next PR After This
Once basic pipeline works, add:
1. Damage pipeline (triggered by attack)
2. Suspension for Shield reaction
3. gRPC streaming for party updates