# Journey 026: Pipelines All The Way Down

## The Moment of Clarity

Kirk: "yeah this pipeline idea and the ability to nest them. like the saving throw pipeline with an input like the kind of saving throw or the attribute to attempt the save on. the nesting of them seems super powerful if done right. i think this is the breakthrough"

That's when everything clicked. We weren't just building a modification pipeline for combat calculations. We were discovering the core abstraction for ALL game mechanics.

## The Evolution of Understanding

### Stage 1: "We need to fix effect modifications"
We started trying to solve the rage problem - effects modifying values through untyped event context was messy and unpredictable.

### Stage 2: "Add a pipeline for calculations"
We thought we'd keep events for reactions and add pipelines just for predictable calculations like damage and attack rolls.

### Stage 3: "Wait, pipelines can be nested"
When working through concentration saves, we realized:
- Attack pipeline triggers damage pipeline
- Damage pipeline triggers concentration save pipeline
- Save pipeline might trigger effect removal
- Effect removal triggers... more pipelines

### Stage 4: "EVERYTHING is a pipeline"
The breakthrough. Every game mechanic can be expressed as a typed pipeline that processes input to output.

## The Core Insight

```go
// Not this:
func HandleAttack(attacker, target Entity) {
    // Hundreds of lines of nested logic
    // Events firing everywhere
    // State mutations all over
}

// But this:
type Pipeline[TInput, TOutput any] interface {
    Process(ctx PipelineContext, input TInput) (TOutput, error)
}

// Every game mechanic becomes a typed, testable pipeline
var AttackPipeline Pipeline[AttackInput, AttackOutput]
var SavePipeline Pipeline[SaveInput, SaveOutput]
var DamagePipeline Pipeline[DamageInput, DamageOutput]
```

## The Power of Composition

### Simple Example: Attack Hits, Deals Damage
```go
attack := AttackPipeline.Process(ctx, AttackInput{
    Attacker: fighter,
    Target: goblin,
    Weapon: sword,
})

if attack.Hit {
    damage := DamagePipeline.Process(ctx, DamageInput{
        Amount: attack.Damage,
        Type: Slashing,
        Target: goblin,
    })
}
```

### But Wait, Damage Triggers Concentration
```go
func (d *DamagePipeline) Process(ctx PipelineContext, input DamageInput) (DamageOutput, error) {
    // Apply damage modifiers
    output := d.calculateDamage(input)
    
    // But damage might trigger concentration!
    if IsConcentrating(input.Target) {
        concSave := ConcentrationPipeline.Process(ctx, ConcentrationInput{
            Caster: input.Target,
            DamageTaken: output.FinalDamage,
        })
        
        if concSave.Failed {
            // Concentration broken, spell ends
        }
    }
    
    return output, nil
}
```

### And Concentration is Just a Save
```go
func (c *ConcentrationPipeline) Process(ctx PipelineContext, input ConcentrationInput) (ConcentrationOutput, error) {
    // Concentration is just a CON save!
    dc := max(10, input.DamageTaken/2)
    
    save := SavePipeline.Process(ctx, SaveInput{
        Ability: Constitution,
        DC: dc,
        Target: input.Caster,
        Reason: "concentration",
    })
    
    return ConcentrationOutput{
        Failed: !save.Success,
    }, nil
}
```

## The Beautiful Recursion

It's not just that pipelines can call other pipelines. It's that EVERYTHING becomes composable:

```go
// A spell cast is a pipeline
SpellCastPipeline
    -> Might trigger CounterspellPipeline
        -> Which triggers SavePipeline
            -> Which triggers ModifierPipeline
    -> If not countered, triggers SpellEffectPipeline
        -> Which might trigger multiple SavePipelines (one per target)
            -> Each save triggers ModifierPipeline
        -> Failed saves trigger DamagePipeline
            -> Which triggers ConcentrationPipeline
                -> Which triggers another SavePipeline

// But each piece is simple and testable!
```

## The Context Thread

The genius part is that context flows through all nested pipelines:

```go
type PipelineContext struct {
    // Game state
    CurrentTurn   EntityID
    Round         int
    
    // Nesting tracking
    Parent        *PipelineContext
    Depth         int
    CallStack     []string  // For debugging!
    
    // Interrupts bubble up
    Interrupts    []Interrupt
    
    // Decisions accumulate
    Decisions     []DecisionMade
}

// You can trace the entire execution!
ctx.CallStack = [
    "CombatRoundPipeline",
    "TurnPipeline[fighter]",
    "ActionPipeline[attack]",
    "AttackPipeline",
    "DamagePipeline",
    "ConcentrationPipeline",
    "SavePipeline[constitution]",
]
```

## Why This Changes Everything

### 1. Type Safety Everywhere
```go
// No more "what's in this event data?"
// Every pipeline has typed input/output
attackResult := AttackPipeline.Process(ctx, AttackInput{...})
// attackResult is typed! IDE knows all fields!
```

### 2. Testability
```go
func TestRageAttackDamage(t *testing.T) {
    // Don't need full game state, just pipeline input
    result := AttackPipeline.Process(testCtx, AttackInput{
        Attacker: ragerWithRage,
        Target: dummy,
        Weapon: greataxe,
    })
    
    assert.Equal(t, result.DamageBonus, 2)
}
```

### 3. Debugging
```go
// Every pipeline can log its stage
[AttackPipeline] Stage: Base - Roll: 15
[AttackPipeline] Stage: Features - Rage adds +2 damage
[AttackPipeline] Stage: Spells - Bless adds 1d4 to hit
[AttackPipeline] Result: Hit, Damage: 1d12+6
[DamagePipeline] Input: 14 slashing
[DamagePipeline] Stage: Resistance - Target has resistance
[DamagePipeline] Result: 7 damage dealt
```

### 4. Modularity
```go
// Want to add a new rule? Add a pipeline stage
// Want to add a new mechanic? Create a pipeline
// Want to modify behavior? Wrap the pipeline

type LoggingPipeline[T, U any] struct {
    Inner Pipeline[T, U]
}

func (l *LoggingPipeline[T, U]) Process(ctx PipelineContext, input T) (U, error) {
    log.Printf("Starting %T with %+v", l.Inner, input)
    result, err := l.Inner.Process(ctx, input)
    log.Printf("Completed with %+v", result)
    return result, err
}
```

## The Unexpected Benefits

### Reactions Become Natural
```go
type InterruptiblePipeline[T, U any] struct {
    Pipeline[T, U]
    InterruptPoints []InterruptPoint
}

func (p *InterruptiblePipeline) Process(ctx PipelineContext, input T) (U, error) {
    for _, point := range p.InterruptPoints {
        if reactions := p.gatherReactions(ctx, point); len(reactions) > 0 {
            for _, reaction := range reactions {
                // Reactions are just pipelines!
                reactionResult := reaction.Pipeline.Process(ctx, reaction.Input)
                if reactionResult.Cancels {
                    return nil, ErrCancelled
                }
            }
        }
    }
    return p.Pipeline.Process(ctx, input)
}
```

### Parallel Processing Possible
```go
// Multiple saves can process in parallel!
results := parallel.Map(targets, func(target Entity) SaveOutput {
    return SavePipeline.Process(ctx, SaveInput{
        Target: target,
        Ability: Wisdom,
        DC: 15,
    })
})
```

### Time Travel Debugging
```go
// Since pipelines are pure functions, we can replay!
recording := RecordedPipeline{
    Input: attackInput,
    Context: ctx.Clone(),
    Timestamp: time.Now(),
}

// Later, replay the exact same calculation
result := AttackPipeline.Process(recording.Context, recording.Input)
```

## The Questions This Raises

1. **Should Actions just return Pipelines?**
   ```go
   type Action interface {
       GetPipeline() Pipeline[ActionInput, ActionOutput]
   }
   ```

2. **Should Effects provide Pipeline stages instead of modifications?**
   ```go
   type Effect interface {
       GetPipelineStages() []PipelineStage
   }
   ```

3. **Is the event bus just for pipeline coordination?**
   ```go
   // Events trigger pipelines, pipelines complete and publish results
   bus.Subscribe("turn.start", func(e Event) {
       TurnPipeline.Process(ctx, e.Entity)
   })
   ```

## The Mental Model Shift

Before: "The game is a series of events that trigger handlers that modify state"

After: "The game is a series of typed pipelines that transform input to output, potentially triggering other pipelines"

This isn't just a refactor. It's a fundamental rethinking of how game mechanics compose.

## What We're Really Building

We're not building a game engine that handles D&D.

We're building a **pipeline composition system** that can express any game's mechanics as typed, testable, composable transformations.

```go
// Any game can define its pipelines
var YahtzeeRollPipeline Pipeline[DiceSet, Score]
var ChessMovePipeline Pipeline[MoveInput, BoardState]
var PokerHandPipeline Pipeline[Cards, HandRank]

// The pattern works for ANY rule system!
```

## The Next Realization

If everything is a pipeline... what's the game loop?

```go
type Game struct {
    Pipeline Pipeline[GameState, GameState]
}

func (g *Game) Run() {
    state := InitialState()
    for !state.IsComplete() {
        state = g.Pipeline.Process(ctx, state)
    }
}
```

The entire game is just a pipeline that transforms game state until completion.

**Pipelines all the way down.**

---

*Sometimes the best abstractions are discovered, not designed. We started trying to fix effect modifications and discovered the fundamental pattern underlying all game mechanics.*