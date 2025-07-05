# Journey 006: Feature Management Pattern

## Context

While implementing the feature system (#33), I created a `FeatureManager` to handle the registration and tracking of features for entities. However, this introduces a different pattern than what we use elsewhere in rpg-toolkit:

- **Conditions**: Applied directly to entities, managed through the event system
- **Resources**: Managed by a `Pool` interface owned by each entity
- **Features**: Currently using a centralized `FeatureManager`

This inconsistency raises the question: what's the right pattern for managing features?

## Current Implementation Analysis

### Features (Current - Centralized Manager)
```go
type FeatureManager interface {
    RegisterFeature(feature Feature) error
    GetFeature(key string) (Feature, bool)
    AddFeature(entity Entity, feature Feature) error
    RemoveFeature(entity Entity, featureKey string) error
    ActivateFeature(entity Entity, featureKey string) error
    GetActiveFeatures(entity Entity) []Feature
}
```

**How it works:**
- Central registry of all possible features
- Tracks which entities have which features
- Manages active/inactive state centrally

### Conditions (Entity-Centric)
```go
type Condition interface {
    Entity
    Target() Entity
    Apply(bus EventBus) error
    Remove(bus EventBus) error
}
```

**How it works:**
- Conditions are entities themselves
- Applied directly to target entities
- No central management - just event system registration
- Persistence would store condition-entity relationships

### Resources (Pool Pattern)
```go
type Pool interface {
    Owner() Entity
    Add(resource Resource) error
    Get(key string) (Resource, bool)
    Consume(key string, amount int, bus EventBus) error
}
```

**How it works:**
- Each entity owns a resource pool
- Pool manages the entity's resources
- No central registry needed

## Options Analysis

### Option 1: Keep Centralized FeatureManager (Current)

**Pros:**
- ✅ Central registry makes it easy to query all available features
- ✅ Level progression logic has a clear home
- ✅ Easy to implement "feature shops" or selection screens
- ✅ Clear separation of concerns

**Cons:**
- ❌ Inconsistent with other modules
- ❌ Requires external state management
- ❌ Harder to persist (need manager state + entity relationships)
- ❌ Single point of failure
- ❌ Not as composable

### Option 2: Entity-Centric (Like Conditions)

**How it would work:**
```go
// Features are owned by entities
type Character struct {
    features []Feature
}

// Features register with event bus when active
feature.Activate(entity, eventBus)
feature.Deactivate(entity, eventBus)
```

**Pros:**
- ✅ Consistent with conditions pattern
- ✅ Features stay with their entities
- ✅ Easy persistence (part of entity data)
- ✅ No central state to manage
- ✅ More composable and testable

**Cons:**
- ❌ No central registry of available features
- ❌ Level progression needs different approach
- ❌ Feature discovery/browsing is harder

### Option 3: Feature Pool (Like Resources)

**How it would work:**
```go
type FeaturePool interface {
    Owner() Entity
    Add(feature Feature) error
    Remove(key string) error
    Get(key string) (Feature, bool)
    Activate(key string, bus EventBus) error
    Deactivate(key string, bus EventBus) error
}
```

**Pros:**
- ✅ Consistent with resources pattern
- ✅ Clear ownership model
- ✅ Encapsulated feature management per entity
- ✅ Could still have a feature registry for available features

**Cons:**
- ❌ Another "pool" type might be confusing
- ❌ Features aren't really "consumed" like resources

### Option 4: Hybrid Approach

**How it would work:**
```go
// Global registry for feature definitions
type FeatureRegistry interface {
    Register(feature Feature) error
    Get(key string) (Feature, bool)
    GetAvailable(entity Entity, level int) []Feature
}

// Features stored on entities
type FeatureHolder interface {
    AddFeature(feature Feature) error
    RemoveFeature(key string) error
    GetFeatures() []Feature
    ActivateFeature(key string, bus EventBus) error
}
```

**Pros:**
- ✅ Best of both worlds
- ✅ Registry for feature discovery
- ✅ Entity ownership for active features
- ✅ Consistent with game needs

**Cons:**
- ❌ More complex
- ❌ Two patterns to understand

## Key Differences Between Systems

### Conditions vs Features
- **Conditions**: Temporary effects, usually applied by external sources
- **Features**: Permanent abilities, part of character identity
- **Conditions**: Often expire or can be removed
- **Features**: Usually permanent once gained

### Resources vs Features  
- **Resources**: Consumable, numeric, regenerate
- **Features**: Abilities, not consumed, provide effects

### Events vs Features
- **Events**: Things that happen
- **Features**: Things you can do

## Decision Factors

1. **Persistence**: How will features be stored?
   - Entity-centric is easier to persist
   - Manager requires separate storage

2. **Game Rules**: How are features gained?
   - Level-up grants need feature discovery
   - Character creation needs available features
   - This suggests some registry is needed

3. **Runtime Performance**: 
   - Entity-centric has better locality
   - Manager has single lookup point

4. **Consistency**: 
   - Should match other patterns where sensible
   - But not at the cost of functionality

## Recommendation

I recommend **Option 4: Hybrid Approach** with entity-centric storage:

1. **FeatureRegistry** (renamed from Manager) only handles:
   - Feature definitions
   - Prerequisites checking  
   - Available features for level/class

2. **Entities** own their features:
   - Features stored on entity
   - Activation/deactivation handled by feature
   - No central tracking of who has what

3. **Pattern similar to Conditions**:
   - Features can register event handlers when active
   - Clean separation of definition vs instance

This gives us:
- Consistency with conditions (entity ownership)
- Registry for game rules (level progression)
- Clean persistence model
- No central state management

## Implementation Changes Needed

1. Remove entity tracking from FeatureManager
2. Rename to FeatureRegistry
3. Add feature storage to entities (game-specific)
4. Move activation logic to features themselves
5. Update examples to show new pattern

## Next Steps

1. Discuss this analysis
2. Make decision on pattern
3. Refactor if needed
4. Document decision in an ADR