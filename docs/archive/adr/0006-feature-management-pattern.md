# ADR-0006: Feature Management Pattern

## Status
Accepted

## Context
While implementing the feature system (#33), we initially created a centralized `FeatureManager` that tracked which entities had which features. This was inconsistent with patterns used elsewhere in rpg-toolkit:

- **Conditions**: Applied directly to entities via the event system
- **Resources**: Managed by a `Pool` owned by each entity
- **Features**: Were using a centralized manager

This inconsistency raised questions about maintainability and the right pattern for the toolkit.

## Decision
We will use a **hybrid approach** for feature management:

1. **FeatureRegistry** - A registry for feature definitions and discovery
2. **Entity-centric storage** - Entities own and manage their features

This means:
- Features are stored on entities (like conditions)
- A registry exists for feature definitions and queries (what's available)
- No central tracking of which entity has which features
- Features self-manage their event registrations when activated

## Rationale

### Why not pure entity-centric (like conditions)?
- Features need discovery: "What features can a level 5 barbarian learn?"
- Character creation needs to browse available features
- Level progression needs to know what's available

### Why not centralized manager?
- Inconsistent with other patterns in the toolkit
- Creates external state that must be persisted separately
- Single point of failure
- Harder to test in isolation

### Why this hybrid approach works
- **Consistency**: Entity ownership matches conditions pattern
- **Discovery**: Registry provides needed queries without state management
- **Persistence**: Features are part of entity data
- **Flexibility**: Games can implement FeatureHolder as needed
- **Testability**: No global state to manage

## Consequences

### Positive
- Consistent with existing patterns
- Easier persistence model
- Better encapsulation
- More flexible for different game implementations
- Clear separation between definitions (registry) and instances (entities)

### Negative
- Two concepts to understand (registry + holder)
- Games must implement FeatureHolder interface
- No built-in tracking of all features across all entities

### Neutral
- Different from traditional manager pattern
- Requires entities to be feature-aware

## Implementation
```go
// For definitions and discovery
type FeatureRegistry interface {
    RegisterFeature(feature Feature) error
    GetFeature(key string) (Feature, bool)
    GetAvailableFeatures(entity Entity, level int) []Feature
}

// For entity feature management
type FeatureHolder interface {
    AddFeature(feature Feature) error
    RemoveFeature(key string) error
    ActivateFeature(key string, bus EventBus) error
    GetActiveFeatures() []Feature
}

// Features handle their own activation
type Feature interface {
    Activate(entity Entity, bus EventBus) error
    Deactivate(entity Entity, bus EventBus) error
}
```

## References
- Journey 006: Feature Management Pattern analysis
- Issue #33: Add feature system
- Conditions module (entity-centric pattern)
- Resources module (pool pattern)