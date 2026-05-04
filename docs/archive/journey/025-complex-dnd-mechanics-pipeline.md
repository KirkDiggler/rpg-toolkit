# Journey 025: Working Through Complex D&D Mechanics for the Pipeline

## Starting With Corrections

Kirk caught me making mistakes about basic D&D mechanics. Let's slow down and work through these carefully.

### Shield Spell - I Was Wrong!

**What I Said (WRONG)**: Shield is a reaction that adds +5 AC after you're hit.

**Actually**: Shield spell is a reaction spell you cast when hit, giving you +5 AC until your next turn. BUT - and this is important - when you cast it as a reaction to being hit, it applies retroactively to that triggering attack! So:

```
1. Attack rolls 18 vs your AC 17 - would hit
2. You cast Shield as reaction 
3. Your AC becomes 22 for this attack (and until your next turn)
4. The 18 now misses!
```

So I was partially right about the retroactive part, but wrong about how Shield works normally. If you already have Shield up, you just have +5 AC in your pipeline - no reaction needed.

### Concentration - Let's Get This Right

**Correct Understanding**:
- Concentration is broken when the CASTER takes damage and fails a Constitution save
- DC = 10 or half damage taken, whichever is higher
- Only the caster makes saves, not the targets of the spell

```go
// Bless with concentration
type BlessEffect struct {
    caster   Entity
    targets  []Entity  // Multiple targets get the benefit
    spellDC  int
}

// When caster takes damage, check concentration
func (b *BlessEffect) Subscribe(bus EventBus) {
    bus.Subscribe("damage.taken", func(e Event) {
        if e.Target == b.caster && e.Damage > 0 {
            dc := max(10, e.Damage/2)
            
            // Trigger a concentration save
            // This is a nested pipeline - damage pipeline triggers save pipeline
            save := ConcentrationSave{
                Caster: b.caster,
                DC: dc,
                Effect: b,
            }
            
            result := savePipeline.Process(save)
            if result.Failed {
                b.RemoveFromAllTargets() // Bless ends for everyone
            }
        }
    })
}
```

## Working Through Each Mechanic Carefully

### 1. Reactions That Actually Do Interrupt

**Counterspell** - Legitimate interrupt:
```go
// Wizard casts Fireball
spellCastPipeline.Start(fireball)
    -> Announce: "Casting Fireball" (but not what level!)
    -> INTERRUPT POINT: Reactions?
    -> Enemy: "I Counterspell"
    -> NOW reveal: "It's level 5"
    -> Counterspell check: 10 + 5 = DC 15
    -> If success: Fireball never happens
    -> If fail: Continue fireball pipeline
```

**Shield** - Reaction with retroactive effect:
```go
// Attack comes in
attackPipeline.Process(attack)
    -> Roll: 18 vs AC 17
    -> Would hit
    -> INTERRUPT POINT: "You would be hit, any reactions?"
    -> Cast Shield (+5 AC until next turn, INCLUDING this attack)
    -> Recalculate: 18 vs AC 22 = miss
```

**Opportunity Attacks** - Reaction to movement:
```go
// Goblin moves away
movement.Process(goblin, newPosition)
    -> Check: Leaving threatened squares?
    -> INTERRUPT POINT: "Triggers opportunity attack"
    -> Fighter: "I attack"
    -> Process attack (which might kill goblin)
    -> If alive, movement continues
```

### 2. Once Per Turn Tracking

**Sneak Attack** - The tricky part:
```go
type SneakAttackEffect struct {
    // NOT just "used this round" 
    // Must track WHOSE turn it was used on
    usedOnTurn map[EntityID]bool
}

// Scenario: Rogue's turn
rogueAttack() -> Use sneak attack
    -> Mark: usedOnTurn[rogue.ID] = true

// Same round, Fighter's turn
fighter.Move() 
    -> Triggers opportunity attack from rogue
    -> Can use sneak attack AGAIN (different turn!)
    -> Mark: usedOnTurn[fighter.ID] = true
```

This means the pipeline needs to know "whose turn is it currently?"

### 3. Post-Roll Decisions

**Divine Smite** - Declared after hit:
```go
attackPipeline.Process(attack)
    -> Roll: Natural 20! (or just hits)
    -> DECISION POINT: "Use Divine Smite?"
    -> If yes: Add smite damage to damage roll
    -> If no: Regular damage
```

**Bardic Inspiration** - Used after seeing roll:
```go
abilityCheckPipeline.Process(check)
    -> Roll: 12
    -> DECISION POINT: "Use Bardic Inspiration?"
    -> If yes: Add 1d8 (rolls 5)
    -> Final: 17
    -> Compare to DC
```

### 4. Target Changes

**Sanctuary** - Forces target change:
```go
type SanctuaryEffect struct {
    protected Entity
}

// In targeting phase
targetingPipeline.Process(attack)
    -> Initial target: Cleric with Sanctuary
    -> Force Wisdom save for attacker
    -> If save fails:
        -> "Choose new target or lose action"
        -> Target changes to Goblin
    -> If save succeeds:
        -> Attack continues against Cleric
```

### 5. Roll Replacement (Not Modification)

**Lucky Feat** - Replace after seeing:
```go
pipeline.Process(roll)
    -> Initial roll: 3 (terrible!)
    -> DECISION POINT: "Use Lucky?"
    -> If yes: Roll new d20: 18
    -> REPLACE original roll entirely
    -> Continue with 18
```

**Portent** - Replace before rolling:
```go
// Divination wizard has predetermined: [3, 18]
pipeline.Process(roll)
    -> BEFORE rolling: "Use Portent?"
    -> If yes: Don't roll, use 18
    -> Roll IS 18, not modified to 18
```

## What This Means for Our Pipeline

### We Need Interrupt Points

```go
type Pipeline struct {
    Stages []Stage
    InterruptPoints map[Stage][]InterruptType
}

const (
    InterruptBeforeRoll     = "before_roll"
    InterruptAfterRoll      = "after_roll"
    InterruptBeforeOutcome  = "before_outcome"
)

func (p *Pipeline) ProcessWithInterrupts(action Action) Result {
    for _, stage := range p.Stages {
        // Check for interrupts before stage
        if interrupts := p.checkInterrupts(stage, InterruptBefore); len(interrupts) > 0 {
            for _, interrupt := range interrupts {
                if interrupt.Cancels() {
                    return CancelledResult{interrupt}
                }
            }
        }
        
        // Process stage
        result := p.processStage(stage, action)
        
        // Check for interrupts after stage
        if interrupts := p.checkInterrupts(stage, InterruptAfter); len(interrupts) > 0 {
            // Some interrupts can modify the result retroactively
            result = p.applyRetroactiveInterrupts(result, interrupts)
        }
    }
}
```

### We Need Context About Turn

```go
type PipelineContext struct {
    CurrentTurn   EntityID  // Whose turn is it?
    Round         int       // What round?
    InitiatorTurn EntityID  // Who started this action?
}
```

### We Need Decision Points

```go
type DecisionPoint struct {
    Stage   Stage
    Prompt  string
    Options []DecisionOption
}

type DecisionOption struct {
    Label    string
    Effect   func(Result) Result
    Resource *Resource  // Optional resource cost
}
```

## The Pattern Still Works!

These complexities don't break the pipeline pattern. They just mean:

1. **Pipelines can be interrupted** - We need interrupt points
2. **Pipelines can be nested** - Damage pipeline triggers save pipeline
3. **Pipelines need context** - Whose turn? What round?
4. **Pipelines can pause for decisions** - "Use Smite?"
5. **Results can be retroactively modified** - Shield spell

The core idea of "stages process in order" still holds. We just need to support these additional capabilities.

## Next Steps

1. Update our pipeline pattern to support interrupts
2. Add context tracking for turn/round
3. Design decision point system
4. Test with complex scenarios like Shield and Counterspell

---

*Slowing down and getting the rules right is important. My initial Shield spell confusion shows why we need to be precise about game mechanics.*