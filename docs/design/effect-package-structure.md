# Effect Package Structure Options

## Components to Place

1. **EffectCore** - Base subscription management and lifecycle
2. **Behavioral Interfaces** - ConditionalEffect, ResourceConsumer, TemporaryEffect, etc.
3. **Integration** - How domain types use these

## Option 1: mechanics/common
```
mechanics/
├── common/
│   ├── effect_core.go
│   ├── subscription_tracker.go
│   └── behaviors/
│       ├── conditional.go
│       ├── resource_consumer.go
│       ├── temporary.go
│       └── property_modifier.go
├── conditions/
│   └── (uses common.EffectCore)
└── proficiency/
    └── (uses common.EffectCore)
```

**Pros:**
- Clear shared location for mechanics
- Easy import path: `mechanics/common`
- Keeps core module focused on entities

**Cons:**
- Another module to manage
- "common" is a bit generic

## Option 2: mechanics/effects
```
mechanics/
├── effects/
│   ├── core.go           // EffectCore
│   ├── tracker.go        // SubscriptionTracker
│   ├── conditional.go
│   ├── resource.go
│   ├── temporary.go
│   └── property.go
├── conditions/
│   └── (uses effects.Core)
└── proficiency/
    └── (uses effects.Core)
```

**Pros:**
- Clear purpose: effect infrastructure
- Good import: `mechanics/effects`
- Natural grouping

**Cons:**
- Might be confused with actual effects

## Option 3: Split by Concern
```
mechanics/
├── subscription/          // Just subscription tracking
│   └── tracker.go
├── behaviors/            // Just behavioral interfaces
│   ├── conditional.go
│   ├── resource.go
│   └── temporary.go
├── conditions/
└── proficiency/
```

**Pros:**
- Single responsibility per package
- Very modular

**Cons:**
- More packages to manage
- More imports needed

## Recommendation: Option 2 (mechanics/effects)

I recommend `mechanics/effects` because:

1. **Clear Purpose**: It's infrastructure for building effects
2. **Good Naming**: `effects.Core`, `effects.ConditionalBehavior`
3. **Logical Grouping**: All effect-related infrastructure together
4. **Future-Proof**: Can add more behaviors without restructuring

Usage would look like:
```go
import "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"

type SimpleProficiency struct {
    effects.Core
    owner   core.Entity
    subject string
}
```

The behavioral interfaces can start in the same package and be split later if needed.