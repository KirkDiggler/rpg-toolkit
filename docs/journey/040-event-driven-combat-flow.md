# Journey 040: Event-Driven Combat Flow

## Date: 2025-08-13

## The Vision: Typed Events, Simple Bus

The event bus is just plumbing. Publishers and subscribers agree on types. The bus doesn't care.

## Core Design

```go
// The bus is dead simple
type EventBus interface {
    Publish(eventType string, data any) error
    Subscribe(eventType string, handler any) string
    Unsubscribe(id string)
}

// Events are just structs - no interface needed!
type AttackDeclaredEvent struct {
    Attacker  core.Entity
    Target    core.Entity
    Weapon    *core.Ref  // "dnd5e:weapon:longsword"
    Timestamp time.Time
}
```

## Full Combat Flow Example

Let's trace a Barbarian with rage attacking an Orc:

### 1. Attack Action Activated

```go
// Player activates attack feature
attackAction.Activate(&ActionContext{
    Actor:  barbarian,
    Target: orc,
    Data: map[string]any{
        "weapon": "dnd5e:weapon:greataxe",
    },
})

// Attack action publishes
bus.Publish("attack.declared", AttackDeclaredEvent{
    Attacker: barbarian,
    Target:   orc,
    Weapon:   ref("dnd5e:weapon:greataxe"),
})
```

### 2. Before Attack Roll - Modifiers Apply

```go
// Bless effect is listening
bus.Subscribe("attack.roll", func(e AttackRollEvent) error {
    if e.Attacker == blessed.Target {
        e.AddModifier("1d4", "bless")  // Event carries mutable data
    }
    return nil
})

// Attack continues, publishes roll event
bus.Publish("attack.roll", AttackRollEvent{
    Attacker:  barbarian,
    Target:    orc,
    BaseRoll:  "1d20+5",
    Modifiers: []Modifier{},  // Bless will add to this
})
```

### 3. Attack Hits - Damage Calculation

```go
// Attack hit, now calculate damage
bus.Publish("damage.calculate", DamageCalculateEvent{
    Source:     barbarian,
    Target:     orc,
    BaseDamage: "1d12+3",  // Greataxe
    DamageType: "slashing",
    Modifiers:  []DamageModifier{},
})

// Rage is listening and adds bonus
bus.Subscribe("damage.calculate", func(e DamageCalculateEvent) error {
    if e.Source == raging.Target {
        e.AddModifier(2, "rage")  // Flat +2 damage
    }
    return nil
})
```

### 4. Before Take Damage - Resistance

```go
// Damage calculated, now applying to target
bus.Publish("damage.before_take", BeforeTakeDamageEvent{
    Target:     orc,
    Amount:     15,  // After all calculations
    DamageType: "slashing",
    Source:     barbarian,
})

// But wait! If orc was raging too...
bus.Subscribe("damage.before_take", func(e BeforeTakeDamageEvent) error {
    if e.Target == raging.Target && e.DamageType.IsPhysical() {
        e.Amount = e.Amount / 2  // Resistance!
    }
    return nil
})
```

### 5. Actual Damage Applied

```go
bus.Publish("damage.taken", DamageTakenEvent{
    Target:       orc,
    FinalAmount:  7,  // After resistance
    OriginalAmount: 15,
    Source:       barbarian,
})
```

## How Features Create Effects

```go
// Rage feature
type Rage struct {
    ref *core.Ref
}

func (r *Rage) Activate(ctx *ActionContext) error {
    // Create the effect
    ragingEffect := effects.New(
        ref("dnd5e:effect:raging"),
        r.ref,  // Source is the rage feature
        ctx.Actor,
    )
    
    // Publish that it's active
    bus.Publish("effect.applied", EffectAppliedEvent{
        Effect: ragingEffect,
        Target: ctx.Actor,
    })
    
    return nil
}
```

## How Effects Hook Into Combat

```go
// When raging effect is applied, it sets up listeners
bus.Subscribe("effect.applied", func(e EffectAppliedEvent) error {
    if e.Effect.Ref().String() == "dnd5e:effect:raging" {
        // Set up damage bonus
        id1 := bus.Subscribe("damage.calculate", func(d DamageCalculateEvent) error {
            if d.Source == e.Target {
                d.AddModifier(2, "rage")
            }
            return nil
        })
        
        // Set up resistance
        id2 := bus.Subscribe("damage.before_take", func(d BeforeTakeDamageEvent) error {
            if d.Target == e.Target && d.DamageType.IsPhysical() {
                d.Amount = d.Amount / 2
            }
            return nil
        })
        
        // Store subscription IDs for cleanup when effect ends
        storeSubscriptions(e.Effect, id1, id2)
    }
    return nil
})
```

## The Beauty: Modifiers Just Work

No modifier interface! Events carry mutable data:

```go
type AttackRollEvent struct {
    Attacker  core.Entity
    Target    core.Entity
    BaseRoll  string
    Modifiers []RollModifier  // Everyone adds to this
}

type RollModifier struct {
    Value  string  // "1d4", "+2", etc
    Source string  // "bless", "rage", etc
}

func (e *AttackRollEvent) AddModifier(value, source string) {
    e.Modifiers = append(e.Modifiers, RollModifier{value, source})
}

// After all subscribers have run, the attack system totals it up
finalRoll := baseRoll + " + " + joinModifiers(e.Modifiers)
```

## Priority via Registration Order

```go
// Register systems in priority order
func SetupCombatSystems(bus EventBus) {
    // Resistance applies last (priority 100)
    bus.Subscribe("damage.before_take", handleResistance)
    
    // Vulnerability applies first (priority 10)  
    bus.Subscribe("damage.before_take", handleVulnerability)
    
    // The bus processes in subscription order
}
```

## What This Gives Us

1. **Type Safety** - Compiler catches mismatched event types
2. **No Interfaces** - Events are just data structs
3. **Mutable Events** - Modifiers add directly to event data
4. **Clean Flow** - Each phase is clear and separate
5. **No Coupling** - Features don't know about effects, effects don't know about each other

## Implementation Notes

The event bus could literally be:

```go
type Bus struct {
    mu        sync.RWMutex
    handlers  map[string][]reflect.Value  // Typed handlers
}

func (b *Bus) Subscribe(eventType string, handler any) string {
    // Store reflect.ValueOf(handler)
    // Return subscription ID
}

func (b *Bus) Publish(eventType string, data any) error {
    // Get handlers for eventType
    // For each handler, call it with reflect
    // handler.Call([]reflect.Value{reflect.ValueOf(data)})
}
```

That's it. Maybe 50 lines total.

## Success Criteria

✅ Can trace full combat with modifiers
✅ Effects hook in via events only  
✅ No complex interfaces
✅ Type safe at compile time
✅ Modifiers just accumulate on events