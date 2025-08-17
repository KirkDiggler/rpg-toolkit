# Journey 027: Rulebooks Own The Pipelines

## The Final Piece Falls Into Place

Kirk: "rulebooks establish the pipelines right? what a powerful tool to use"

THIS. This is where our architecture becomes truly powerful. The toolkit doesn't know D&D or Pathfinder or any specific game - it just provides the pipeline infrastructure. Each rulebook defines its own pipelines.

## The Separation of Concerns Perfected

### Toolkit Provides Infrastructure
```go
// rpg-toolkit/core/pipeline/types.go
type Pipeline[TInput, TOutput any] interface {
    Process(ctx PipelineContext, input TInput) (TOutput, error)
}

type PipelineStage[T any] interface {
    Name() string
    Process(ctx PipelineContext, value T) (T, error)
}

type PipelineContext struct {
    CurrentTurn   EntityID
    Round         int
    Parent        *PipelineContext
    Depth         int
    // ... core context that any game needs
}
```

### Rulebooks Define Their Pipelines
```go
// rulebooks/dnd5e/pipelines/attack.go
var AttackPipeline = pipeline.New[AttackInput, AttackOutput](
    pipeline.Stages{
        BaseRollStage{},        // d20 + proficiency + ability
        EquipmentStage{},       // Magic weapon bonuses
        ClassFeatureStage{},    // Fighting styles, etc
        SpellEffectStage{},     // Bless, Bane, etc
        AdvantageStage{},       // Roll twice, take higher/lower
        CriticalStage{},        // Natural 20s and 1s
    },
)

// rulebooks/pathfinder2e/pipelines/attack.go  
var AttackPipeline = pipeline.New[AttackInput, AttackOutput](
    pipeline.Stages{
        MultipleAttackPenalty{}, // Pathfinder-specific!
        BaseRollStage{},         
        ItemBonusStage{},       
        StatusBonusStage{},     
        CircumstanceStage{},    // Different bonus stacking!
        FortuneStage{},         // Different advantage system!
        CriticalStage{},        // Different crit rules!
    },
)
```

## Each Game's Rules Expressed as Pipelines

### D&D 5e Death Saves
```go
// rulebooks/dnd5e/pipelines/death.go
var DeathSavePipeline = pipeline.New[DeathSaveInput, DeathSaveOutput](
    pipeline.Stages{
        RollD20Stage{},
        CheckNatural20{},      // Instant revival with 1 HP
        CheckNatural1{},       // Counts as 2 failures  
        CompareToTen{},        // 10+ is success
        TrackSuccesses{},      // 3 successes = stable
        TrackFailures{},       // 3 failures = death
    },
)
```

### Pathfinder 2e Dying Rules (Completely Different!)
```go
// rulebooks/pathfinder2e/pipelines/dying.go
var DyingPipeline = pipeline.New[DyingInput, DyingOutput](
    pipeline.Stages{
        CheckWounded{},         // Wounded condition affects dying
        IncreaseDying{},        // Dying is a value, not saves
        RecoveryCheck{},        // DC 10 + dying value
        ReduceDyingOnSuccess{}, // Success reduces dying by 1
        CheckDyingFour{},       // Dying 4 = death
    },
)
```

### Call of Cthulhu Sanity Checks
```go
// rulebooks/coc7e/pipelines/sanity.go
var SanityPipeline = pipeline.New[SanityInput, SanityOutput](
    pipeline.Stages{
        RollPercentile{},       // d100 system
        CompareToSanity{},      // Roll under current sanity
        CalculateLoss{},        // Different loss for success/fail
        CheckBreakingPoint{},   // Triggers madness
        BoutOfMadness{},        // Temporary or indefinite
    },
)
```

## The Power of This Approach

### 1. Games Define Their Own Rules
```go
// D&D 5e has advantage/disadvantage
type AdvantageStage struct{}
func (a AdvantageStage) Process(ctx PipelineContext, roll Roll) (Roll, error) {
    if roll.HasAdvantage {
        roll.Result = max(rollD20(), rollD20())
    }
    return roll, nil
}

// Pathfinder 2e has fortune/misfortune  
type FortuneStage struct{}
func (f FortuneStage) Process(ctx PipelineContext, roll Roll) (Roll, error) {
    // Different mechanic, same pipeline pattern
}
```

### 2. Toolkit Doesn't Need Game Knowledge
```go
// The toolkit just processes pipelines
func (e *Engine) Execute[T, U any](p Pipeline[T, U], input T) (U, error) {
    ctx := e.createContext()
    return p.Process(ctx, input)
}

// It doesn't know or care about D&D vs Pathfinder
```

### 3. Mixing Systems Becomes Possible
```go
// Want to use Pathfinder crit rules in D&D?
var HouseRuleAttackPipeline = pipeline.New[AttackInput, AttackOutput](
    dnd5e.BaseRollStage{},
    dnd5e.EquipmentStage{},
    dnd5e.SpellEffectStage{},
    pathfinder.CriticalStage{},  // Mix and match!
)
```

### 4. Custom Rules Are Just New Pipelines
```go
// Your homebrew game
var CustomSpellcastPipeline = pipeline.New[SpellInput, SpellOutput](
    MyCustomManaStage{},        // Your mana system
    dnd5e.ConcentrationStage{},  // Reuse D&D concentration
    MyCustomBackfireStage{},     // Your special rule
)
```

## The Architecture Crystallizes

```
rpg-toolkit/
├── core/
│   └── pipeline/           # Pipeline infrastructure
│       ├── types.go        # Core interfaces
│       ├── context.go      # PipelineContext
│       ├── executor.go     # Pipeline execution
│       └── composer.go     # Pipeline composition
│
├── mechanics/              # Reusable pipeline stages
│   ├── dice/              # Dice rolling stages
│   ├── modifiers/         # Modifier calculation stages
│   └── conditions/        # Condition checking stages
│
└── rulebooks/
    ├── dnd5e/
    │   └── pipelines/     # D&D 5e specific pipelines
    │       ├── attack.go
    │       ├── saves.go
    │       ├── death.go
    │       └── skills.go
    │
    ├── pathfinder2e/
    │   └── pipelines/     # Pathfinder specific pipelines
    │
    └── coc7e/
        └── pipelines/     # Call of Cthulhu pipelines
```

## The Beautiful Symmetry

Remember our original principle: **"Toolkit provides infrastructure, games provide implementation"**

Now it's perfectly expressed:
- **Toolkit**: Here's how to build and compose pipelines
- **Rulebook**: Here are my game's specific pipelines
- **Features/Effects**: Here are stages for those pipelines

## Example: Implementing a New Game

```go
// rulebooks/vampire5e/pipelines/hunger.go
package vampire5e

// Vampire has hunger dice that can cause bestial failures
var HungerRollPipeline = pipeline.New[HungerInput, HungerOutput](
    RollNormalDice{},        // Roll regular dice
    RollHungerDice{},        // Roll hunger dice (different color)
    CheckForOnes{},          // 1s on hunger dice are bad
    BestialFailure{},        // Special failure condition
    MessyCritical{},         // Special critical condition
)

// The toolkit handles the execution, Vampire defines the rules
```

## This Solves Our Original Problem

Remember rage? Now it's simple:

```go
// D&D 5e defines the attack pipeline with a Features stage
// Rage just provides a stage implementation
type RageStage struct {
    effect *RageEffect
}

func (r RageStage) Process(ctx PipelineContext, attack AttackValue) (AttackValue, error) {
    if r.effect.IsActive() && attack.IsMelee() {
        attack.DamageBonus += r.effect.DamageBonus()
    }
    return attack, nil
}

// That's it. Rage is now ~20 lines instead of 300+
```

## The Meta Realization

We're not building a game engine.

We're building a **game rule expression system** where:
- Rules are pipelines
- Pipelines are composable
- Games define their pipelines
- Toolkit just executes them

Any tabletop RPG's rules can be expressed as pipelines. The toolkit doesn't need to know the rules - it just needs to know how to run pipelines.

---

*The journey from "effects need better modification handling" to "games are collections of typed pipelines" shows how the best architectures emerge from solving real problems.*