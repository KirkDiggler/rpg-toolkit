# Pipeline Examples: Verifying Assumptions

**Status: DESIGN VERIFICATION**

*Companion to: [Pipeline Execution Patterns](pipeline-execution-patterns.md)*

## Example 1: Simple Combat Round

Let's trace through a complete combat round to verify our pipeline assumptions.

### Scenario
Fighter attacks Wizard who is concentrating on a spell. The wizard has Shield prepared as a reaction.

### Execution Trace

```go
// Round starts
CombatRoundPipeline.Process(ctx, RoundInput{Round: 3})
    │
    ├─ TurnPipeline.Process(ctx.Nest("fighter-turn"), TurnInput{Entity: fighter})
    │   │
    │   ├─ ActionPipeline.Process(ctx.Nest("attack-action"), ActionInput{Type: "attack"})
    │   │   │
    │   │   ├─ AttackPipeline.Process(ctx.Nest("attack"), AttackInput{
    │   │   │       Attacker: fighter,
    │   │   │       Target: wizard,
    │   │   │       Weapon: sword
    │   │   │   })
    │   │   │   │
    │   │   │   ├─ [INTERRUPT: BeforeRoll] // No reactions
    │   │   │   │
    │   │   │   ├─ [SEQUENTIAL] BaseRollStage: d20 = 15
    │   │   │   ├─ [SEQUENTIAL] ProficiencyStage: 15 + 3 = 18
    │   │   │   ├─ [SEQUENTIAL] ModifierStage: 18 + 2 = 20 (strength)
    │   │   │   │
    │   │   │   ├─ [INTERRUPT: AfterRoll] Shield Reaction Available!
    │   │   │   │   └─ ShieldPipeline.Process(ctx.Nest("shield"), ShieldInput{
    │   │   │   │           Caster: wizard,
    │   │   │   │           IncomingAttack: 20,
    │   │   │   │           CurrentAC: 15
    │   │   │   │       })
    │   │   │   │       ├─ Decision: Player chooses YES
    │   │   │   │       ├─ SpellSlotPipeline.Process(...) // Consumes slot
    │   │   │   │       └─ Return: ModifyResult{TargetAC: +5}
    │   │   │   │
    │   │   │   ├─ Attack vs AC: 20 vs 20 (15 + 5 from Shield) = HIT!
    │   │   │   │
    │   │   │   └─ [NESTED] DamagePipeline.Process(ctx.Nest("damage"), DamageInput{
    │   │   │           Amount: "1d8+2",
    │   │   │           Type: "slashing",
    │   │   │           Target: wizard
    │   │   │       })
    │   │   │       ├─ [SEQUENTIAL] RollStage: 1d8 = 6
    │   │   │       ├─ [SEQUENTIAL] ModifierStage: 6 + 2 = 8
    │   │   │       ├─ [SEQUENTIAL] ResistanceStage: No resistance
    │   │   │       ├─ [SEQUENTIAL] ApplyStage: wizard.hp -= 8
    │   │   │       │
    │   │   │       └─ [NESTED] ConcentrationPipeline.Process(ctx.Nest("concentration"), 
    │   │   │               ConcentrationInput{
    │   │   │                   Caster: wizard,
    │   │   │                   DamageTaken: 8
    │   │   │               })
    │   │   │               ├─ CalculateDC: max(10, 8/2) = 10
    │   │   │               │
    │   │   │               └─ [NESTED] SavePipeline.Process(ctx.Nest("con-save"),
    │   │   │                       SaveInput{
    │   │   │                           Target: wizard,
    │   │   │                           Ability: CON,
    │   │   │                           DC: 10
    │   │   │                       })
    │   │   │                       ├─ [SEQUENTIAL] BaseStage: d20 = 12
    │   │   │                       ├─ [SEQUENTIAL] AbilityStage: 12 + 1 = 13
    │   │   │                       ├─ [SEQUENTIAL] ProficiencyStage: 13 + 2 = 15
    │   │   │                       └─ Return: SaveOutput{Success: true, Roll: 15}
    │   │   │               │
    │   │   │               └─ Return: ConcentrationOutput{Maintained: true}
    │   │   │       │
    │   │   │       └─ Return: DamageOutput{
    │   │   │               FinalDamage: 8,
    │   │   │               ConcentrationMaintained: true
    │   │   │           }
    │   │   │   │
    │   │   │   └─ Return: AttackOutput{
    │   │   │           Hit: true,
    │   │   │           Damage: 8,
    │   │   │           TargetUsedReaction: true
    │   │   │       }
    │   │   │
    │   │   └─ Return: ActionOutput{Completed: true}
    │   │
    │   └─ Return: TurnOutput{Completed: true}
    │
    └─ Return: RoundOutput{Completed: true}
```

### Context Stack at Deepest Point

```go
ctx.CallStack = [
    "CombatRoundPipeline",
    "fighter-turn",
    "attack-action", 
    "attack",
    "damage",
    "concentration",
    "con-save"
]

ctx.Depth = 7
ctx.Results = {
    "shield": {used: true, acBonus: 5},
    "damage": {dealt: 8},
    "save": {success: true, roll: 15},
    "concentration": {maintained: true}
}
```

## Example 2: Counterspell Chain

Verifying that interrupts can trigger their own interrupts.

### Scenario
Wizard A casts Fireball. Wizard B attempts to Counterspell. Wizard A attempts to Counterspell the Counterspell!

```go
SpellCastPipeline.Process(ctx, SpellCastInput{
    Caster: wizardA,
    Spell: "fireball",
    Level: 3
})
    │
    ├─ [INTERRUPT: BeforeCast] Counterspell Available!
    │   └─ CounterspellPipeline.Process(ctx.Nest("counterspell-1"), CounterspellInput{
    │           Caster: wizardB,
    │           TargetSpell: "fireball",
    │           TargetLevel: 3,
    │           CounterLevel: 3
    │       })
    │       │
    │       ├─ [INTERRUPT: BeforeCast] Counter-Counterspell Available!
    │       │   └─ CounterspellPipeline.Process(ctx.Nest("counterspell-2"), 
    │       │           CounterspellInput{
    │       │               Caster: wizardA,
    │       │               TargetSpell: "counterspell",
    │       │               TargetLevel: 3,
    │       │               CounterLevel: 4  // Using 4th level!
    │       │           })
    │       │           ├─ AutoSuccess: CounterLevel (4) > TargetLevel (3)
    │       │           └─ Return: CancelResult{Cancelled: true}
    │       │
    │       └─ Return: CancelResult{Cancelled: false} // Was itself countered!
    │
    ├─ Fireball proceeds normally...
    └─ [NESTED] Multiple SavePipelines for each target...
```

**✅ Verification**: Interrupts can be interrupted, creating complex but traceable chains.

## Example 3: Death Saves with Interrupts

Verifying state tracking across pipeline executions.

### Scenario
Character drops to 0 HP, makes death saves, gets healed, drops again.

```go
// Character drops to 0 HP
DamagePipeline.Process(ctx, DamageInput{Target: fighter, Amount: 25})
    └─ DeathPipeline.Process(ctx.Nest("death-check"), DeathInput{Entity: fighter})
        ├─ SetState: fighter.deathSaves = {successes: 0, failures: 0}
        └─ Return: DeathOutput{Unconscious: true, Dying: true}

// Turn 1: Death Save
DeathSavePipeline.Process(ctx, DeathSaveInput{Entity: fighter})
    ├─ SavePipeline.Process(ctx.Nest("death-save"), SaveInput{DC: 10, NoModifiers: true})
    │   └─ Roll: 14 = Success
    ├─ UpdateState: fighter.deathSaves.successes = 1
    └─ Return: DeathSaveOutput{Success: true, Total: {1, 0}}

// Turn 2: Death Save  
DeathSavePipeline.Process(ctx, DeathSaveInput{Entity: fighter})
    ├─ Roll: 8 = Failure
    ├─ UpdateState: fighter.deathSaves.failures = 1
    └─ Return: DeathSaveOutput{Success: false, Total: {1, 1}}

// Turn 3: Healing interrupts death!
HealingPipeline.Process(ctx, HealingInput{Target: fighter, Amount: 5})
    ├─ [INTERRUPT: OnHealing] CheckDying
    │   └─ ResetDeathSaves: fighter.deathSaves = {0, 0}
    ├─ ApplyHealing: fighter.hp = 5
    └─ Return: HealingOutput{Stabilized: true, Conscious: true}

// Later: Drops again - death saves reset!
DamagePipeline.Process(ctx, DamageInput{Target: fighter, Amount: 10})
    └─ DeathPipeline.Process(ctx.Nest("death-check"), DeathInput{Entity: fighter})
        ├─ SetState: fighter.deathSaves = {successes: 0, failures: 0} // Reset!
        └─ Return: DeathOutput{Unconscious: true, Dying: true}
```

**✅ Verification**: State can be tracked across multiple pipeline executions with proper reset logic.

## Example 4: Advantage/Disadvantage Resolution

Verifying complex modifier interactions.

### Scenario
Rogue attacking from hiding (advantage) against a prone target (advantage) while poisoned (disadvantage) and blessed.

```go
AttackPipeline.Process(ctx, AttackInput{Attacker: rogue, Target: goblin})
    │
    ├─ [SEQUENTIAL] GatherModifiersStage
    │   ├─ Hidden: +Advantage
    │   ├─ TargetProne: +Advantage  
    │   ├─ Poisoned: +Disadvantage
    │   └─ Blessed: +1d4
    │
    ├─ [SEQUENTIAL] ResolveAdvantageStage
    │   ├─ Advantages: 2
    │   ├─ Disadvantages: 1
    │   └─ Result: Advantage (they don't stack, just need one of each to cancel)
    │
    ├─ [SEQUENTIAL] RollStage
    │   ├─ Roll 1: d20 = 12
    │   ├─ Roll 2: d20 = 18 (due to advantage)
    │   ├─ Bless: 1d4 = 3
    │   └─ Final: 18 + 3 = 21
    │
    └─ Return: AttackOutput{Roll: 21, HadAdvantage: true}
```

**✅ Verification**: Multiple modifiers can be collected and resolved in a predictable order.

## Example 5: Legendary Resistance

Verifying interrupt can override normal pipeline result.

### Scenario
Dragon fails a save but uses Legendary Resistance to succeed instead.

```go
SavePipeline.Process(ctx, SaveInput{Target: dragon, Ability: WIS, DC: 17})
    │
    ├─ [SEQUENTIAL] Stages result in: 14 (failure)
    │
    ├─ [INTERRUPT: AfterRoll] CheckLegendaryResistance
    │   └─ LegendaryResistancePipeline.Process(ctx.Nest("legendary"), 
    │           LegendaryInput{
    │               Entity: dragon,
    │               FailedSave: true,
    │               SaveDC: 17
    │           })
    │           ├─ Check: dragon.legendaryResistances > 0 ✓
    │           ├─ Decision: AI chooses to use
    │           ├─ Update: dragon.legendaryResistances -= 1
    │           └─ Return: ReplaceResult{Output: SaveOutput{Success: true}}
    │
    └─ Return: SaveOutput{Success: true, UsedLegendary: true}
```

**✅ Verification**: Interrupts can completely replace pipeline results.

## Key Insights from Examples

### 1. Context is King
The context object carries everything needed:
- Execution trace (debugging)
- Shared state (round number, current turn)
- Results from nested pipelines
- Interrupt registrations

### 2. Pipelines are Composable Legos
Each pipeline is self-contained but can:
- Trigger other pipelines (nested)
- Be interrupted by other pipelines
- Modify other pipeline inputs/outputs
- Replace other pipeline results entirely

### 3. State Lives Outside Pipelines
Pipelines transform input→output but don't hold state:
- Entity state (HP, conditions) lives in entities
- Game state (round, turn) lives in context
- Pipeline state is only temporary during execution

### 4. Decisions are First-Class
Player/AI decisions are explicit:
- Shield reaction: "Do you want to cast?"
- Legendary resistance: "Do you want to use?"
- Counterspell: "What level slot?"

These become part of the pipeline input or interrupt data.

### 5. Everything is Traceable
The complete execution can be reconstructed:
```go
// At any point in execution:
log.Printf("Current pipeline: %s", ctx.CallStack[len(ctx.CallStack)-1])
log.Printf("Depth: %d", ctx.Depth)
log.Printf("Parent results: %+v", ctx.Parent.Results)

// After execution:
log.Printf("Full trace: %s", strings.Join(ctx.CallStack, " → "))
```

## Verification Summary

✅ **Nested pipelines maintain isolation while sharing context**
✅ **Interrupts can meaningfully modify execution flow**  
✅ **Complex chains (counterspell-counterspell) are manageable**
✅ **State tracking works across pipeline boundaries**
✅ **Multiple modifiers compose predictably**
✅ **Decisions integrate naturally into flow**
✅ **Everything remains traceable and debuggable**

## The "Aha!" Moment

The real power isn't in any single pattern, but in how they combine:

```go
// This one line can trigger an entire game's worth of mechanics:
result := ActionPipeline.Process(ctx, ActionInput{Type: "attack", Target: dragon})

// But each piece is simple, tested, and typed!
```

The pipeline pattern doesn't just organize code - it makes complex game mechanics *composable* and *predictable*.