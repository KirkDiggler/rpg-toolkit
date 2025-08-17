# Journey 024: From Event Chaos to Modification Pipeline

## The Starting Point

After attempting to implement rage (PR #221), we realized our effects system had fundamental issues:
- Magic strings everywhere
- Unclear modification order
- Lost context of what modified what
- No way to selectively load effects

But more importantly, I (@kirk) had a gut feeling something was off. Effects were using events to modify things, but it felt fuzzy. When does rage modify damage? How do multiple effects interact?

## The Initial Overreaction

Claude first suggested abandoning events entirely for a pull-based system:

```go
// Instead of events, query effects when needed
modifiers := effectManager.GetModifiersFor(attacker, ModifierTypeAttack)
```

But I pushed back - **hard**. The event system is how we decouple things! Traps listen to movement, curses react to specific priest interactions. This decoupling is magical and powerful.

## The Real Problem Emerges

Through discussion, we identified the actual issues:

### 1. Order of Operations
```go
// Who goes first? 
bus.Subscribe("attack.roll", priority=10, rageHandler)
bus.Subscribe("attack.roll", priority=10, blessHandler)  // Same priority!
```

### 2. Lost Modification Context
```go
// Rage adds +2 damage
event.Data[DataKeyDamageBonus] = 2

// But Brutal Critical needs to know what's base vs bonus
// Context is lost in flat key-value pairs
```

### 3. No Selective Loading
```go
// Can't ask for "just attack modifiers"
effectManager.GetEffects() // Returns everything
```

## The Conversation Breakthrough

Kirk: "The Stage is defined to process all stages together so our rulebook would say we do items features... some order of things. Our Modification we add would declare the stage our ordering will process in the order by stage?"

That's when it clicked. We don't need to abandon events. We need a **modification pipeline** that works WITH events.

## The Pipeline Pattern

### Rulebook Defines Order
```go
// rulebooks/dnd5e/combat/pipeline.go
var AttackPipeline = Pipeline{
    Stages: []Stage{
        StageBase,       // Weapon damage
        StageEquipment,  // Magic weapons
        StageFeatures,   // Rage, Sneak Attack
        StageSpells,     // Bless, Hex
        StageConditions, // Exhaustion
        StageFinal,      // Critical hits
    },
}
```

### Effects Declare Their Stage
```go
func (r *RageEffect) GetModifications(ctx context.Context, action Action) []Modification {
    return []Modification{
        {
            Stage: StageFeatures,  // I'm a feature!
            Type:  ModifierDamageBonus,
            Value: r.damageBonus(),
            Source: "rage",
        },
    }
}
```

### Processing is Deterministic
```go
// Process modifications in pipeline order
for _, stage := range pipeline.Stages {
    mods := getModsForStage(stage)
    result = applyMods(result, mods)
    
    // Can track/debug each stage
    bus.Publish(StageCompleteEvent{stage, result, mods})
}
```

## The Beautiful Part

Events still handle the reactive, decoupled magic:
```go
// Curse still listens for priest interaction
curse.Subscribe("entity.interact", func(e Event) {
    if priest.Name == "Brother Thomas" {
        curse.Remove()
    }
})

// Trap still triggers on movement
trap.Subscribe("entity.move", func(e Event) {
    if e.Position.Near(trap) {
        trap.Trigger()
    }
})
```

But calculations use the pipeline for predictability:
```go
// Attack calculation is deterministic
result := combatPipeline.Calculate(AttackCalculation{
    Base: weapon.Damage,
    Attacker: barbarian,
})
```

## Different Actions, Different Pipelines

Each rulebook can define pipelines for different actions:
```go
var SavingThrowPipeline = Pipeline{
    Stages: []Stage{
        StageBase,        // Ability modifier
        StageProficiency, // Proficiency bonus
        StageFeatures,    // Paladin's aura
        StageSpells,      // Bless, Resistance
        StageItems,       // Cloak of Protection
    },
}
```

## The Key Insights

1. **Keep events for decoupling** - They're perfect for reactions and triggers
2. **Add pipeline for calculations** - Predictable order for modifications
3. **Rulebook owns the order** - Not priority numbers or registration order
4. **Effects just declare where they fit** - "I'm a feature" not "I'm priority 23"
5. **Modifications are tracked** - Know what modified what and why

## Why This Feels Right

- **It's both flexible AND predictable** - Events for magic, pipeline for math
- **It matches how games work** - D&D has explicit ordering rules
- **It's debuggable** - Can see each stage's modifications
- **It's testable** - Pipeline doesn't need full event bus
- **It keeps our decoupling** - Traps and curses still work beautifully

## What We Learned

Sometimes resistance to change is good. My pushback on abandoning events led us to a better solution that keeps what works while fixing what doesn't. The event system's decoupling is too valuable to lose.

The real problem wasn't events - it was unstructured modifications. The pipeline gives us structure without sacrificing flexibility.

## Next Steps

1. Write ADR for modification pipeline pattern
2. Create issues for implementation:
   - Core pipeline infrastructure
   - Modification types and stages
   - Integration with existing effects
   - Rulebook-specific pipelines
3. Refactor rage to use the new pattern as proof of concept

## The Bigger Picture

This pattern might apply beyond combat:
- Spell slot calculations
- Movement speed calculations  
- Skill check modifications
- AC calculations

Anywhere we need predictable, debuggable modification of values.

---

*Sometimes the best solutions come from pushing back on the first idea. The tension between "we need order" and "we need flexibility" led us to "why not both?"*