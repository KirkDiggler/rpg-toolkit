# ADR-0020: Features and Conditions Simplification

## Status
Accepted

## The Problem in One Sentence

Implementing a simple feature like Rage requires understanding 14 interface methods, while it should just be ~50 lines of actual game logic.

## Context

After implementing core.Ref and core.Source across several modules, we've identified significant architectural issues in the features and conditions modules:

1. **Interface Bloat**: Feature interface has 14 methods, violating Interface Segregation Principle
2. **Builder Pattern Issues**: Conditions use builders that accumulate errors instead of failing fast
3. **Duplicate Infrastructure**: Both modules reimplement what effects.Core already provides
4. **String Proliferation**: String identifiers everywhere instead of typed references
5. **Event Filtering**: Every feature hears ALL events and checks relevance manually

Additionally, we explored abstracting everything to "actions" but pulled back - keeping domain concepts clear is more important than premature abstraction.

## Decision

### 1. Simplify Feature to Essential Methods

The Feature interface should focus on what features actually do:

```go
type Feature interface {
    // Identity
    Ref() *core.Ref
    Name() string
    Description() string
    
    // Activation
    NeedsTarget() bool  // UI: should we ask for a target?
    Activate(owner core.Entity, opts ...ActivateOption) error
    IsActive() bool
    
    // Events  
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}

// Activation options
type ActivateOption func(*ActivateContext)

func WithTarget(target core.Entity) ActivateOption {
    return func(ctx *ActivateContext) {
        ctx.Target = target
    }
}

// Features with limited uses implement this
type FeatureWithResources interface {
    Feature
    GetResource() resources.Resource  // Let the resource system handle it
}

// SimpleFeature embeds effects.Core for common functionality
type SimpleFeature struct {
    *effects.Core
    ref         *core.Ref
    name        string
    description string
    // Feature-specific fields only
}
```

This is 6 essential methods instead of 14, focused on the core responsibilities. Features that have limited uses expose that through the resource system.

### 2. Replace Builders with Options Pattern

Kill the builder pattern entirely. Use variadic options that validate immediately:

```go
// Options validate on construction
func WithTarget(entity core.Entity) ConditionOption {
    return func(c *SimpleCondition) error {
        if entity == nil {
            return fmt.Errorf("target cannot be nil")
        }
        c.target = entity
        return nil
    }
}

// Usage - fails immediately if invalid
condition, err := NewCondition(
    WithRef(conditions.Poisoned), // Using a constant!
    WithTarget(entity),
    WithDuration(10),
)
```

### 3. Persistence Pattern: ToJSON/LoadFromJSON

Features handle their own persistence with simple JSON serialization:

```go
// Each feature type knows how to save itself
func (r *RageFeature) ToJSON() json.RawMessage {
    data := RageData{
        Ref:           "dnd5e:feature:rage",
        UsesRemaining: r.usesRemaining,
        IsActive:      r.isActive,
    }
    return json.Marshal(data)
}

// Loading uses a simple switch - no magic registries
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
```

### 4. Smart Event Subscriptions

Instead of every feature checking every event:

```go
// Old way - manual filtering in every handler
func handleDamage(e Event) {
    if e.Target() != me {
        return  // Not my damage, ignore
    }
    // ...
}

// New way - filter at subscription
bus.On(events.EventBeforeTakeDamage).
    ToTarget(myID).  // Only MY damage events!
    Do(func(e Event) {
        // I know this is my damage
        applyResistance(e)
    })
```

### 5. Keep Domain Concepts Clear

We explored "everything is actions" but decided against it:
- Features are features
- Spells are spells  
- Conditions are conditions

Keeping domain concepts clear makes the code more understandable. We can generalize later if patterns emerge.

## Consequences

### Positive
- **Massive code reduction**: From 14 methods to 6, focus on game logic
- **Smart filtering**: Event bus filters at subscription level
- **Simple persistence**: ToJSON/LoadFromJSON pattern is clear
- **Dirty tracking**: Efficient saves only when needed
- **Fail-fast**: Options validate immediately, not at build time
- **Breaking changes are explicit**: No silent failures from legacy paths
- **Clear domain concepts**: Features are features, not generic actions

### Negative  
- **Breaking changes**: All existing feature/condition code must be updated
- **Migration effort**: Need to update all consuming code
- **No registry magic**: Simple switch statements might seem primitive

### Neutral
- **Embedded effects.Core**: Composition provides shared functionality
- **Feature owns everything**: Ref, implementation, and loader live together

## Implementation Order

1. Update Feature interface to ~8 essential methods
2. Implement SimpleFeature with effects.Core embedding
3. Add ToJSON/LoadFromJSON pattern for persistence
4. Replace condition builder with options
5. Update event bus to support smart subscriptions
6. Migrate existing features to new pattern

## Success Metric

**Success is measured by how simple it is to implement complex things.**

The true test isn't whether we can implement Rage - it's whether implementing Rage is so simple that we can focus on its actual game mechanics rather than fighting the framework.

## Examples

### Before (Current - 14 methods)
```go
type Feature interface {
    GetID() string
    GetType() string
    Key() string
    Name() string
    Description() string
    Type() string
    Level() int
    Source() string
    IsActive() bool
    CanTrigger(event events.GameEvent) bool
    TriggerFeature(event events.GameEvent) error
    GetEventListeners() []events.Listener
    GetResources() []resources.Resource
    GetModifiers() []events.Modifier
    GetProficiencies() []proficiency.Proficiency
    // ... and more
}
```

### After (Proposed - ~8 methods)
```go
// Complete Rage implementation in ~50 lines
type RageFeature struct {
    *features.SimpleFeature
    usesRemaining int
    maxUses       int
    isActive      bool
    dirty         bool
}

func (r *RageFeature) NeedsTarget() bool {
    return false  // Rage targets self
}

func (r *RageFeature) Activate(owner core.Entity, opts ...ActivateOption) error {
    if r.isActive {
        return ErrAlreadyActive
    }
    if r.usesRemaining <= 0 {
        return ErrNoUsesRemaining
    }
    
    r.usesRemaining--
    r.isActive = true
    r.dirty = true
    
    // Rage affects the owner (self-targeting)
    event := events.NewGameEvent("feature.activate", owner, owner)
    event.Context().Set("feature_ref", RageRef)
    return r.eventBus.Publish(context.Background(), event)
}

// Fireball needs a target
func (f *FireballSpell) NeedsTarget() bool {
    return true
}

func (f *FireballSpell) Activate(owner core.Entity, opts ...ActivateOption) error {
    ctx := parseOptions(opts...)
    if ctx.Target == nil {
        return ErrTargetRequired
    }
    
    // Cast fireball at the target
    event := events.NewGameEvent("spell.cast", owner, ctx.Target)
    event.Context().Set("spell", "fireball")
    return f.bus.Publish(context.Background(), event)
}

// Rage exposes its uses through the resource system
func (r *RageFeature) GetResource() resources.Resource {
    return r.rageResource  // A CountResource with current/max
}

func (r *RageFeature) ToJSON() json.RawMessage {
    return json.Marshal(map[string]interface{}{
        "ref":            "dnd5e:feature:rage",
        "uses_remaining": r.usesRemaining,
        "is_active":      r.isActive,
    })
}