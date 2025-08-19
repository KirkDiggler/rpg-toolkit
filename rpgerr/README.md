# rpgerr: Errors That Tell The Whole Story Automatically

## The Magic: Context Accumulation

```go
// As your attack flows through the game systems, each layer adds its context
ctx = rpgerr.WithMetadata(ctx, 
    rpgerr.Meta("round", 3),
    rpgerr.Meta("attacker", "barbarian-001"))

// Deeper in the attack pipeline
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("weapon", "greataxe"),
    rpgerr.Meta("target_ac", 19))

// When something goes wrong, the error captures the ENTIRE journey
if distance > weaponReach {
    return rpgerr.OutOfRangeCtx(ctx, "melee attack")
    // Error contains: round=3, attacker=barbarian-001, weapon=greataxe, target_ac=19
    // You didn't pass these values - they accumulated automatically!
}
```

**THE INSIGHT:** Your errors accumulate context as they flow through your systems. No manual passing, no forgotten details - the complete story tells itself.

## Why This Matters

Traditional error handling loses context at every boundary:

```go
// Traditional: Context gets lost
func Attack(attacker, target string) error {
    if err := checkRange(); err != nil {
        return fmt.Errorf("attack failed: %w", err) // Lost: who attacked whom
    }
}

func checkRange() error {
    return errors.New("out of range") // No context about the attack
}
```

With rpgerr, context accumulates automatically:

```go
// rpgerr: Context accumulates through the journey
func Attack(ctx context.Context, attacker, target string) error {
    ctx = rpgerr.WithMetadata(ctx,
        rpgerr.Meta("attacker", attacker),
        rpgerr.Meta("target", target))
    
    if err := checkRange(ctx); err != nil {
        return rpgerr.WrapCtx(ctx, err, "attack failed") // Preserves everything
    }
}

func checkRange(ctx context.Context) error {
    ctx = rpgerr.WithMetadata(ctx,
        rpgerr.Meta("distance", 14.5),
        rpgerr.Meta("max_range", 5))
    
    return rpgerr.OutOfRangeCtx(ctx, "melee attack") 
    // Error now contains: attacker, target, distance, max_range
}
```

## Real Combat Example

Watch how a complete attack failure story builds itself:

```go
// Combat system adds encounter context
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("encounter_id", "enc-001"),
    rpgerr.Meta("round", 3),
    rpgerr.Meta("turn", "fighter"))

// Attack action adds attacker context
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("action_type", "attack"),
    rpgerr.Meta("attacker_id", "fighter-001"),
    rpgerr.Meta("target_id", "goblin-002"))

// Range validation adds position context
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("attacker_position", "5,5"),
    rpgerr.Meta("target_position", "15,15"),
    rpgerr.Meta("weapon", "shortsword"),
    rpgerr.Meta("weapon_reach", 5),
    rpgerr.Meta("calculated_distance", 14.14))

// The error tells the complete story
err := rpgerr.OutOfRangeCtx(ctx, "melee attack")

// err.Meta now contains:
// - encounter_id: "enc-001"
// - round: 3
// - turn: "fighter"
// - attacker_id: "fighter-001"
// - target_id: "goblin-002"
// - weapon: "shortsword"
// - calculated_distance: 14.14
// - weapon_reach: 5
// Every layer's context, automatically captured!
```

## Deep Pipeline Example: Damage Resistance

See how errors accumulate through nested pipelines:

```go
// AttackPipeline level
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("pipeline", "AttackPipeline"),
    rpgerr.Meta("attacker", "barbarian-001"),
    rpgerr.Meta("target", "dragon-001"))

// HitCalculation level  
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("pipeline", "HitCalculation"),
    rpgerr.Meta("attack_roll", 18),
    rpgerr.Meta("total_attack", 25),
    rpgerr.Meta("target_ac", 19),
    rpgerr.Meta("hit", true))

// DamagePipeline level
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("pipeline", "DamagePipeline"),
    rpgerr.Meta("base_damage", "1d12"),
    rpgerr.Meta("damage_roll", 8))

// DamageReduction level discovers resistance
ctx = rpgerr.WithMetadata(ctx,
    rpgerr.Meta("pipeline", "DamageReduction"),
    rpgerr.Meta("damage_type", "slashing"),
    rpgerr.Meta("target_resistances", []string{"slashing", "piercing"}))

err := rpgerr.BlockedCtx(ctx, "resistance to non-magical slashing")

// The error contains the ENTIRE attack flow:
// - Who attacked (barbarian-001)
// - What they hit (dragon-001)  
// - The roll that hit (25 vs AC 19)
// - The damage rolled (8 on 1d12)
// - Why it was reduced (slashing resistance)
// Four levels deep, zero manual passing!
```

## Core Concepts

### 1. Context Accumulation
Every layer adds its piece of the story:
```go
ctx = rpgerr.WithMetadata(ctx, rpgerr.Meta("key", "value"))
```

### 2. Context-Aware Errors
Use `Ctx` variants to capture accumulated context:
```go
rpgerr.WrapCtx(ctx, err, "message")           // Wrap with context
rpgerr.OutOfRangeCtx(ctx, "melee attack")     // Game-specific with context
rpgerr.ResourceExhaustedCtx(ctx, "spell slots") // Resource errors with context
```

### 3. Error Metadata Access
Retrieve the accumulated story:
```go
meta := rpgerr.GetMeta(err)
attacker := meta["attacker"].(string)
round := meta["round"].(int)
```

## Game-Specific Error Types

rpgerr provides error codes for common RPG scenarios:

- `CodeNotAllowed` - Action prohibited by rules
- `CodePrerequisiteNotMet` - Missing requirements (level, class, feat)
- `CodeResourceExhausted` - Out of resources (HP, spell slots, actions)
- `CodeOutOfRange` - Target too far
- `CodeInvalidTarget` - Cannot target that entity
- `CodeConflictingState` - States conflict (rage + concentration)
- `CodeTimingRestriction` - Wrong phase/turn
- `CodeCooldownActive` - Ability on cooldown
- `CodeImmune` - Target immune to effect
- `CodeBlocked` - Action blocked by effect
- `CodeInterrupted` - Action interrupted by reaction

## The Power of Automatic Accumulation

The beauty of rpgerr is what you DON'T have to do:

- ❌ No manual error decoration at each level
- ❌ No passing error context through function parameters
- ❌ No lost information at system boundaries
- ❌ No reconstructing what happened from logs

Instead:
- ✅ Context accumulates naturally as execution flows
- ✅ Errors contain the complete journey automatically
- ✅ Every system layer's contribution is preserved
- ✅ Debugging shows exactly what happened and why

## Installation

```bash
go get github.com/KirkDiggler/rpg-toolkit/rpgerr
```

## Quick Start

```go
import "github.com/KirkDiggler/rpg-toolkit/rpgerr"

// Add context at each system layer
ctx = rpgerr.WithMetadata(ctx, 
    rpgerr.Meta("system", "combat"),
    rpgerr.Meta("round", 1))

// Create errors that capture everything
if err := doSomething(ctx); err != nil {
    return rpgerr.WrapCtx(ctx, err, "combat action failed")
}

// Or create new errors with full context
if !canAct {
    return rpgerr.TimingRestrictionCtx(ctx, "not your turn")
}
```

The error tells the whole story. Automatically.