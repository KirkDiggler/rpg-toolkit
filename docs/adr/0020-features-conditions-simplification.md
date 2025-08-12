# ADR-0020: Features and Conditions Simplification

## Status
Proposed

## The Problem in One Sentence

Implementing a simple feature like Rage requires understanding 14 interface methods, while it should just be a function that subscribes to events.

## Context

After implementing core.Ref and core.Source across several modules, we've identified significant architectural issues in the features and conditions modules:

1. **Interface Bloat**: Feature interface has 14 methods, violating Interface Segregation Principle
2. **Builder Pattern Issues**: Conditions use builders that accumulate errors instead of failing fast
3. **Duplicate Infrastructure**: Both modules reimplement what effects.Core already provides
4. **String Proliferation**: String identifiers everywhere instead of typed references

Additionally, we're seeing patterns emerge where we need standard references (like "core:class:barbarian") used consistently across the codebase.

## Decision

### 1. Simplify Feature to Its Essence

Features ARE effects with metadata. They should embed effects.Core directly:

```go
// Feature is just the core contract
type Feature interface {
    core.Entity         // GetID(), GetType()  
    Ref() *core.Ref     // What feature is this?
    Source() *core.Source // What granted this?
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
}

// SimpleFeature embeds effects.Core for all the heavy lifting
type SimpleFeature struct {
    *effects.Core
    ref   *core.Ref
    level int
    // Feature-specific fields only
}
```

Optional behaviors through interface composition:
```go
type FeatureWithPrerequisites interface {
    Feature
    MeetsPrerequisites(entity core.Entity) bool
}

type FeatureWithResources interface {
    Feature
    GetResources() []resources.Resource
}
```

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

### 3. Provide Standard Ref Constants

Create packages that export common Refs as constants:

```go
// Package: mechanics/features/standard
package standard

var (
    // Class features
    Rage = core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "feature", 
        Value:  "rage",
    })
    
    SneakAttack = core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "feature",
        Value:  "sneak_attack",
    })
)

// Package: mechanics/conditions/standard  
package standard

var (
    // Conditions
    Poisoned = core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "condition",
        Value:  "poisoned",
    })
    
    Stunned = core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "condition",
        Value:  "stunned",
    })
)

// Package: core/classes
package classes

var (
    Barbarian = &core.Source{
        Category: core.SourceClass,
        Name:     "barbarian",
    }
    
    Fighter = &core.Source{
        Category: core.SourceClass,
        Name:     "fighter",
    }
)
```

Usage becomes type-safe and clean:
```go
feat := features.NewSimple(
    features.WithRef(standard.Rage),
    features.FromSource(classes.Barbarian),
    features.AtLevel(1),
)
```

### 4. Unify Through effects.Core

Both SimpleFeature and SimpleCondition embed effects.Core:
- Automatic subscription tracking
- Consistent activation/deactivation
- Source tracking built-in
- Event handling infrastructure

## Consequences

### Positive
- **Massive code reduction**: Remove duplicate infrastructure
- **Type safety**: Constants prevent typos in strings
- **Fail-fast**: Options validate immediately, not at build time
- **Consistency**: All effects-based mechanics work the same way
- **Discoverability**: IDEs can autocomplete standard refs
- **Breaking changes are explicit**: No silent failures from legacy paths

### Negative  
- **Breaking changes**: All existing feature/condition code must be updated
- **Migration effort**: Need to update all consuming code
- **New packages**: Need to maintain standard ref constants

### Neutral
- **More packages**: standard refs live in separate packages, but provide clear organization
- **Explicit imports**: Must import ref packages, but makes dependencies clear

## Implementation Order

1. Create standard ref packages with common constants
2. Refactor Feature interface to minimal core
3. Implement SimpleFeature with effects.Core
4. Replace condition builder with options
5. Update SimpleCondition to use effects.Core
6. Migrate existing implementations

## Success Metric

**Success is measured by how simple it is to implement complex things.**

The true test isn't whether we can implement Rage - it's whether implementing Rage is so simple that we can focus on its actual game mechanics rather than fighting the framework.

## Examples

### Before (Current)
```go
// Strings everywhere, no validation
feat := &SomeFeature{
    key:    "rage",        // Could be typo
    source: "barbarian",   // What kind of source?
}

// Builder accumulates errors
builder := NewConditionBuilder("poisoned")
builder.WithTarget(entity)
condition, err := builder.Build() // Fails here
```

### After (Proposed)
```go
// Type-safe constants
feat := features.NewSimple(
    features.WithRef(standard.Rage),      // Compile-time checked
    features.FromSource(classes.Barbarian), // Clear source type
)

// Options fail immediately
condition, err := conditions.NewSimple(
    conditions.WithRef(standard.Poisoned), // Type-safe constant
    conditions.WithTarget(entity),         // Validates immediately
)

// The real test - implementing complex features simply
func NewRage() *SimpleFeature {
    return NewSimple(
        WithRef(refs.Rage),
        FromSource(sources.ClassBarbarian),
        OnApply(func(f *SimpleFeature, bus events.EventBus) error {
            // Focus on WHAT rage does, not HOW to wire it up
            // This is the complexity we care about
            return nil
        }),
    )
}