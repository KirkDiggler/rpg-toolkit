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
func NewRage() *SimpleFeature {
    return NewSimple(
        WithRef(refs.Rage),
        FromSource(sources.ClassBarbarian),
        AtLevel(1),
        OnApply(func(f *SimpleFeature, bus events.EventBus) error {
            // Just the rage logic, nothing else
            return nil
        }),
    )
}
```

## Assumptions Becoming Clearer

Through this journey, our assumptions are crystallizing:

1. **Features are just effects with metadata** - They subscribe to events and modify game state
2. **Identity, implementation, and loading live together** - The barbarian package knows everything about barbarian features
3. **The toolkit provides infrastructure, games provide features** - We give them SimpleFeature, they give us Rage
4. **Event bus is the nervous system** - Features don't call each other, they react to events
5. **Data loading is feature-specific** - Each feature knows how to reconstruct itself from saved data

The add-on module question (like Artificer) is interesting but can wait. For now, we're seeing that if each module owns its features completely, extension becomes natural - just import another package that exports its own features with their own loaders.

When we implement Fireball, it should be equally obvious:
```go
func NewFireball() *SimpleSpell {
    return NewSimple(
        WithRef(refs.Fireball),
        AtLevel(3),
        OnCast(func(s *SimpleSpell, targets []Entity) error {
            // Just the fireball logic
            return nil
        }),
    )
}
```

If we achieve this level of simplicity while maintaining extensibility, we've succeeded. The toolkit should make the hard things possible and the simple things trivial.