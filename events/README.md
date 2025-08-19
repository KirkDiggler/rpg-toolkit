# Events - The '.On(bus)' Pattern

THE MAGIC: Explicit topic-to-bus connections that make event flow beautiful.

```go
// The pattern that changes everything
attacks := combat.AttackTopic.On(bus)  // SEE the connection happen
attacks.Subscribe(ctx, handleAttack)   // Type-safe from here
```

## The Journey Pattern

Watch how an attack flows through your game, accumulating modifications:

```go
// 1. Attack begins its journey
attack := AttackEvent{AttackerID: "barbarian", Damage: 10}
chain := NewStagedChain[AttackEvent](stages)

// 2. Connect to the bus - explicit and beautiful
attacks := combat.AttackChain.On(bus)

// 3. Journey through all features (rage adds +2, bless adds +4)
modifiedChain, _ := attacks.PublishWithChain(ctx, attack, chain)

// 4. Execute accumulated modifications
result, _ := modifiedChain.Execute(ctx, attack)
// Base: 10 → Rage: 12 → Bless: 16
```

Every feature along the journey sees the attack and can add its modifier. The chain accumulates them all, then applies them in order. This is the infrastructure that makes modifier stacking work.

## Why '.On(bus)' Is Special

Traditional event systems hide the connection:
```go
// Where does this go? Who knows?
bus.Publish("attack", event)  // String-based, error-prone
```

Our pattern makes it explicit:
```go
// Crystal clear where this connects
attacks := AttackTopic.On(bus)  // See it, understand it
attacks.Publish(ctx, event)     // Type-safe, IDE-friendly
```

## Two Patterns, Two Journeys

### Pure Notifications - One-Way Journey

Events that notify systems without modification:

```go
// Define once at compile-time
var LevelUpTopic = DefineTypedTopic[LevelUpEvent]("player.levelup")

// Connect at runtime
levelups := LevelUpTopic.On(bus)

// Achievement system listens
levelups.Subscribe(ctx, func(ctx context.Context, e LevelUpEvent) error {
    if e.NewLevel == 10 {
        UnlockAchievement(e.PlayerID, "Decimator")
    }
    return nil
})

// UI system listens
levelups.Subscribe(ctx, func(ctx context.Context, e LevelUpEvent) error {
    ShowLevelUpAnimation(e.PlayerID, e.NewLevel)
    return nil
})

// Publish once, flows everywhere
levelups.Publish(ctx, LevelUpEvent{PlayerID: "hero", NewLevel: 10})
```

### Chained Events - Accumulation Journey

Events that gather modifications as they travel:

```go
// Define the journey stages
const (
    StageBase       = "base"       // Starting values
    StageFeatures   = "features"   // Class features apply
    StageConditions = "conditions" // Status effects apply
    StageEquipment  = "equipment"  // Gear bonuses apply
    StageFinal      = "final"      // Last-minute adjustments
)

// Rage feature adds to the journey
func (r *Rage) Apply(bus EventBus) error {
    attacks := AttackChain.On(bus)  // Connect to journey
    
    attacks.SubscribeWithChain(ctx, func(ctx context.Context, e AttackEvent, c Chain[AttackEvent]) (Chain[AttackEvent], error) {
        if e.AttackerID == r.ownerID {
            // Add our modifier to the conditions stage
            c.Add(StageConditions, "rage", func(ctx context.Context, e AttackEvent) (AttackEvent, error) {
                e.Damage += 2
                return e, nil
            })
        }
        return c, nil
    })
}

// Bless spell adds to the journey
func (b *Bless) Apply(bus EventBus) error {
    attacks := AttackChain.On(bus)  // Same journey, different feature
    
    attacks.SubscribeWithChain(ctx, func(ctx context.Context, e AttackEvent, c Chain[AttackEvent]) (Chain[AttackEvent], error) {
        if b.affects(e.AttackerID) {
            c.Add(StageConditions, "bless", func(ctx context.Context, e AttackEvent) (AttackEvent, error) {
                e.Damage += 4
                return e, nil
            })
        }
        return c, nil
    })
}
```

## Complete Journey Example

Here's how damage flows through an entire combat system:

```go
package example

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/core/chain"
)

// Topic definitions - compile-time constants
var (
    AttackChain = events.DefineChainedTopic[AttackEvent]("combat.attack")
    DamageChain = events.DefineChainedTopic[DamageEvent]("combat.damage")
    SaveChain   = events.DefineChainedTopic[SaveEvent]("combat.save")
)

// The attack journey through the system
func ResolveAttack(bus events.EventBus, attacker, target Entity) {
    ctx := context.Background()
    
    // 1. Attack Roll Journey
    attack := AttackEvent{
        AttackerID: attacker.ID,
        TargetID:   target.ID,
        BaseRoll:   roll(20),
    }
    
    attackChain := events.NewStagedChain[AttackEvent](stages)
    attacks := AttackChain.On(bus)
    
    // Journey through all attack modifiers
    modifiedChain, _ := attacks.PublishWithChain(ctx, attack, attackChain)
    attack, _ = modifiedChain.Execute(ctx, attack)
    
    if attack.TotalRoll < target.AC {
        return // Miss - journey ends
    }
    
    // 2. Damage Calculation Journey
    damage := DamageEvent{
        SourceID: attacker.ID,
        TargetID: target.ID,
        Amount:   roll(8) + attacker.StrMod,
        Type:     "slashing",
    }
    
    damageChain := events.NewStagedChain[DamageEvent](stages)
    damages := DamageChain.On(bus)
    
    // Journey through all damage modifiers
    modifiedChain, _ = damages.PublishWithChain(ctx, damage, damageChain)
    damage, _ = modifiedChain.Execute(ctx, damage)
    
    // 3. Saving Throw Journey (if applicable)
    if damage.RequiresSave {
        save := SaveEvent{
            TargetID:   target.ID,
            SaveType:   damage.SaveType,
            DC:         damage.SaveDC,
            BaseRoll:   roll(20),
        }
        
        saveChain := events.NewStagedChain[SaveEvent](stages)
        saves := SaveChain.On(bus)
        
        // Journey through all save modifiers
        modifiedChain, _ = saves.PublishWithChain(ctx, save, saveChain)
        save, _ = modifiedChain.Execute(ctx, save)
        
        if save.Success {
            damage.Amount /= 2  // Half damage on save
        }
    }
    
    // Apply final damage
    target.HP -= damage.Amount
}
```

See how each event has its own journey through the system? Features can hook into any point, adding their modifiers. The infrastructure handles the accumulation and ordering.

## Key Insights

1. **The '.On(bus)' pattern** - Makes connections explicit and discoverable
2. **Journey-driven design** - Events flow through systems, accumulating changes
3. **Compile-time topics, runtime connections** - Static safety with dynamic flexibility
4. **Infrastructure, not implementation** - We provide the roads, games provide the cars

## API at a Glance

```go
// Define topics (compile-time)
var AttackTopic = DefineTypedTopic[AttackEvent]("combat.attack")
var AttackChain = DefineChainedTopic[AttackEvent]("combat.attack")

// Connect to bus (runtime)
attacks := AttackTopic.On(bus)  // THE MAGIC MOMENT

// Use with type safety
attacks.Subscribe(ctx, handler)
attacks.Publish(ctx, event)

// For chains
chain := NewStagedChain[AttackEvent](stages)
modifiedChain, _ := attacks.PublishWithChain(ctx, event, chain)
result, _ := modifiedChain.Execute(ctx, event)
```

## This Is Infrastructure

We don't implement game rules. We provide the infrastructure for events to journey through your game systems. The '.On(bus)' pattern makes these journeys explicit, type-safe, and beautiful.

Your features define what happens. Our infrastructure ensures it happens in the right order, with the right types, accumulating properly along the way.