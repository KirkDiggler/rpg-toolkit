# Journey 008: Features and Conditions Refactor

## Date: 2025-08-12

## The Story

This is our 3rd or 4th attempt at implementing the event bus and feature system. Each iteration has taught us something crucial:

1. **First attempt**: Would have been crazy complex to implement. Too much abstraction, too many layers.
2. **Second attempt**: Created the toolkit separation - this really helped clarify responsibilities.
3. **Third attempt**: Got us to where we were before today - better, but still had interface bloat and builder complexity.
4. **Today**: We finally see the path to simplicity.

## Success Criteria

**Success will be measured by how easy it is to implement features and spells.**

If implementing Rage or Fireball requires understanding 14 methods and a builder pattern, we've failed. If it's a simple function with clear options, we've succeeded. Simple and extensible - that's our measure.

## The Problem

After successfully updating resources and proficiency modules to use `core.Ref` and `core.Source`, we turned our attention to the more complex features and conditions modules. What we found was concerning:

### Features Module Issues

The Feature interface has grown to **14 methods**. This is a clear violation of the Interface Segregation Principle. The interface mixes:
- Identity (Key, Name, Description)
- Metadata (Type, Level, Source)
- State management (IsActive, Activate, Deactivate)
- Event handling (CanTrigger, TriggerFeature, GetEventListeners)
- Resource management (GetResources, GetModifiers, GetProficiencies)
- Prerequisites (HasPrerequisites, MeetsPrerequisites, GetPrerequisites)

This is too much. Features trying to be everything means they're good at nothing.

### Conditions Module Issues

The conditions module uses a builder pattern that:
- Accumulates errors instead of failing fast
- Requires a final Build() call to validate
- Feels very un-Go-like

Go prefers simple constructors with validation upfront, not builders that delay validation until the end.

## The Core Insight

Looking at proficiency's SimpleProficiency, we see it delegates to `effects.Core` for common functionality. This pattern works well because:
- It provides consistent behavior across effect types
- It handles subscription management cleanly
- It tracks activation state uniformly

But features and conditions predate this pattern. They're doing too much themselves.

## Proposed Solution

### 1. Split Feature Interface

Instead of one giant interface, split by responsibility:

```go
// Feature is just the core identity and metadata
type Feature interface {
    core.Entity  // GetID(), GetType()
    Name() string
    Description() string
    Level() int
    Source() *core.Source
}

// FeatureWithResources adds resource management
type FeatureWithResources interface {
    Feature
    GetResources() []resources.Resource
}

// FeatureWithModifiers adds modifier support
type FeatureWithModifiers interface {
    Feature
    GetModifiers() []events.Modifier
}

// FeatureWithPrerequisites adds prerequisite checking
type FeatureWithPrerequisites interface {
    Feature
    MeetsPrerequisites(entity core.Entity) bool
}

// ActiveFeature adds activation behavior
type ActiveFeature interface {
    Feature
    IsActive() bool
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
}
```

This way, implementations only implement what they actually need.

### 2. Replace Builders with Options

Instead of:
```go
builder := NewConditionBuilder(Poisoned).
    WithTarget(entity).
    WithSource("spider_bite").
    WithDuration(10)
condition, err := builder.Build()
```

Use:
```go
condition := NewCondition(
    Poisoned,
    WithTarget(entity),
    WithSource(&core.Source{Category: core.SourceCreature, Name: "spider"}),
    WithDuration(10),
)
// Fails immediately if invalid
```

### 3. Leverage effects.Core

Both features and conditions should embed `effects.Core` like SimpleProficiency does:

```go
type SimpleFeature struct {
    *effects.Core
    name        string
    description string
    level       int
    // feature-specific fields
}

type SimpleCondition struct {
    *effects.Core
    target    core.Entity
    condition string
    // condition-specific fields
}
```

This gives us:
- Consistent activation/deactivation
- Automatic subscription tracking
- Source tracking via effects.Core

### 4. Use core.Ref Everywhere

Replace string identifiers with `*core.Ref`:
- Feature keys become Refs (e.g., `dnd5e:feature:rage`)
- Condition types become Refs (e.g., `dnd5e:condition:poisoned`)
- Subject references use Refs consistently

## Breaking Changes Are Good

We're not deprecating - we're breaking. This is good because:
1. We'll know immediately what needs updating
2. No legacy code paths to maintain
3. Forces clean migration

## Implementation Order

1. **Update effects.Core first** (already done!)
2. **Features module**
   - Split the interface
   - Create SimpleFeature using effects.Core
   - Replace builders with option functions
   - Update to use core.Ref
3. **Conditions module**
   - Remove builder pattern
   - Create SimpleCondition using effects.Core
   - Use option functions
   - Update to use core.Ref

## The Philosophy

We're not adding abstraction - we're removing it. The goal is to make the simple cases simple and the complex cases possible. Most features and conditions are simple - they shouldn't need to implement 14 methods to exist.

By splitting interfaces and using composition over inheritance, we get:
- Cleaner code
- Better testability
- More flexibility
- Less boilerplate

## Next Steps

Start with features since it has the most egregious interface bloat. Once we prove the pattern there, conditions will follow naturally.

Remember: We're building for real complex behaviors. Every simplification now pays dividends when we're deep in combat resolution or spell interaction logic later.

## The Measure of Success

When we implement Rage, it should look like this:

```go
// The complete Rage implementation
type RageFeature struct {
    *features.SimpleFeature  // Embeds effects.Core
    usesRemaining int
    maxUses       int
    isActive      bool
}

func (r *RageFeature) Activate(target core.Entity) error {
    if r.isActive {
        return AlreadyActiveError()
    }
    if r.usesRemaining <= 0 {
        return NoUsesError()
    }
    
    r.usesRemaining--
    r.isActive = true
    r.dirty = true
    
    // Fire event - rage handles itself through subscriptions
    event := events.NewGameEvent("feature.activate", r.owner, nil)
    event.Context().Set("feature_ref", RageRef)
    return r.eventBus.Publish(context.Background(), event)
}

func (r *RageFeature) ToJSON() json.RawMessage {
    data := RageData{
        Ref:           "dnd5e:feature:rage",
        UsesRemaining: r.usesRemaining,
        IsActive:      r.isActive,
    }
    return json.Marshal(data)
}
```

## How It Works in Practice

```go
// Loading from database - simple switch on ref
func LoadFeatureFromJSON(data json.RawMessage) (Feature, error) {
    var peek struct {
        Ref string `json:"ref"`
    }
    json.Unmarshal(data, &peek)
    
    switch peek.Ref {
    case "dnd5e:feature:rage":
        return barbarian.LoadRageFromJSON(data)
    case "dnd5e:feature:second_wind":
        return fighter.LoadSecondWindFromJSON(data)
    default:
        return nil, fmt.Errorf("unknown feature: %s", peek.Ref)
    }
}

// Character activation
func (c *Character) ActivateFeature(ref string) error {
    for _, feature := range c.Features {
        if feature.Ref().String() == ref {
            return feature.Activate(nil)
        }
    }
    return ErrFeatureNotFound
}

// End of turn - save dirty features
func (c *Character) EndTurn() error {
    for _, feat := range c.features {
        if feat.IsDirty() {
            database.Save(feat.ToJSON())
            feat.MarkClean()
        }
    }
}
```

## Chapter 4: Smart Event Subscriptions

One more problem emerged - features were hearing ALL events and checking relevance:

```go
// Every handler was doing this
func handleDamage(e Event) {
    if e.Target() != me {
        return  // Not my damage, ignore
    }
    // ...
}
```

The solution? Make the event bus smart:

```go
// Subscribe with filters
bus.On(events.EventBeforeTakeDamage).
    ToTarget(myID).  // Only MY damage!
    Do(func(e Event) {
        // I know this is my damage
        applyResistance(e)
    })
```

## Chapter 5: Features vs Actions vs Effects

Late one night, we had a revelation - "Everything is just Actions!" But then reality hit:

```go
// Too abstract!
type Character struct {
    actions map[string]Action  // What even is anything?
}
```

We pulled back. Features are features, spells are spells. Keep the domain clear:

```go
type Character struct {
    Features   []Feature    // Clear
    Spells     []Spell      // Organized  
    Conditions []Condition  // Understandable
}
```

The lesson: Don't over-abstract too early. Ship something that works, learn, then maybe generalize.

## Chapter 6: The Final Design

### The Interface
```go
type Feature interface {
    // Identity
    Ref() *core.Ref
    Name() string
    
    // Activation
    CanActivate() bool
    Activate(target core.Entity) error
    
    // Events
    Apply(events.EventBus) error
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
}
```

### Key Patterns We Settled On

1. **Features own everything** - Ref, implementation, and loader live together
2. **Simple switch for loading** - No magic registries
3. **ToJSON for storage, ToData for internals** - Clear separation
4. **Smart event subscriptions** - Filter at the bus level
5. **Keep domain concepts clear** - Don't abstract to "actions" yet

## The Final Test: Implementing Complex Features

Here's what Second Wind looks like:

```go
type SecondWindFeature struct {
    *features.SimpleFeature
    used bool
}

func (s *SecondWindFeature) Activate(target core.Entity) error {
    if s.used {
        return AlreadyUsedError()
    }
    
    // Heal 1d10 + level
    healing := dice.D10(1).GetValue() + s.level
    s.owner.Heal(healing)
    
    s.used = true
    s.dirty = true
    
    return nil
}

func (s *SecondWindFeature) ToJSON() json.RawMessage {
    return json.Marshal(map[string]interface{}{
        "ref":  "dnd5e:feature:second_wind",
        "used": s.used,
    })
}
```

And here's a spell with targeting:

```go
type FireballSpell struct {
    *spells.SimpleSpell
}

func (f *FireballSpell) NeedsTarget() bool {
    return true
}

func (f *FireballSpell) Activate(target core.Entity) error {
    if !f.owner.HasSpellSlot(3) {
        return NoSpellSlotsError()
    }
    
    f.owner.UseSpellSlot(3)
    
    // Fire the spell event
    event := events.NewGameEvent("spell.cast", f.owner, target)
    event.Context().Set("spell", "fireball")
    event.Context().Set("damage", "8d6")
    event.Context().Set("save_dc", f.calculateDC())
    
    return f.bus.Publish(context.Background(), event)
}
```

## Assumptions Becoming Clearer

Through this journey, our assumptions crystallized:

1. **Features are just effects with metadata** - They subscribe to events and modify game state
2. **Identity, implementation, and loading live together** - The barbarian package knows everything about barbarian features
3. **The toolkit provides infrastructure, games provide features** - We give them SimpleFeature, they give us Rage
4. **Event bus is the nervous system** - Features don't call each other, they react to events
5. **Keep it simple for alpha** - Ship features as features, learn, then maybe generalize

## The Measure of Success

**How simple is it to implement complex features?**

With our final design, Rage is ~50 lines of actual logic. No boilerplate, no 14-method interface. Just the game mechanics.

The toolkit makes the hard things possible and the simple things trivial. Mission accomplished.