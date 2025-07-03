# ADR-0003: Conditions as Entities

Date: 2025-01-03

## Status

Proposed

## Context

In designing the conditions system for the RPG toolkit, we need to decide how conditions (status effects like "blessed", "poisoned", "cursed") fit into our architecture.

Key requirements:
- Conditions must persist across sessions (e.g., a curse lasting weeks)
- Conditions need to be queryable and inspectable
- Conditions modify game behavior through the event system
- Conditions can have complex state and duration rules

### Options Considered

1. **Conditions as Pure Event Handlers**
   - Simple functions that subscribe to events
   - Managed entirely by a registry

2. **Conditions as Entities**
   - First-class game objects implementing core.Entity
   - Have their own identity and lifecycle
   - Can be stored and retrieved like other entities

3. **Conditions as Separate Domain Objects**
   - Their own interface hierarchy
   - Not entities, but condition-specific types

## Decision

We will implement **Conditions as Entities** that participate in the event system.

### Design

```go
type Condition interface {
    core.Entity  // Conditions ARE entities in our game world
    
    // Relationships
    Target() core.Entity    // What entity has this condition
    Source() string         // What caused this condition
    
    // Lifecycle
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
    IsActive() bool
    
    // Duration
    ExpiresAt() *time.Time
    OnTick(ctx context.Context, bus events.EventBus) (bool, error)
}
```

### Why Conditions are Entities

1. **Persistence**: Conditions need to save/load across sessions
2. **Identity**: Each condition instance needs unique identification
3. **Queryability**: "What conditions affect this character?"
4. **Consistency**: Everything in the game world is an entity
5. **Extensibility**: Conditions can have arbitrarily complex state

## Consequences

### Positive

- Conditions can persist naturally through the entity storage system
- Rich querying: find conditions by type, target, source, etc.
- Conditions can emit their own events ("condition_applied", "condition_triggered")
- Complex conditions (progressive diseases, multi-stage curses) are possible
- Consistent with "everything is an entity" mental model

### Negative

- More complex than simple event handlers
- Need unique IDs for every condition instance
- Storage system must handle condition entities
- More setup required for testing

### Neutral

- Conditions will need their own repository/storage patterns
- Entity interface methods may need better names (noted separately)

## Implementation Notes

### Entity Interface Naming

The current `core.Entity` interface uses generic method names:
- `GetID()` - could be `EntityID()` or `Identifier()`
- `GetType()` - could be `EntityType()` or `Kind()`

This should be addressed separately as it affects all entities.

### Storage Considerations

Conditions as entities means they'll need:
- Serialization/deserialization
- Foreign key relationships (condition -> target entity)
- Cleanup when target entities are deleted
- Migration strategy for condition format changes

## Example

```go
// A curse that lasts until removed by Remove Curse spell
type CurseCondition struct {
    id         string
    target     core.Entity
    curseType  string
    power      int
    appliedAt  time.Time
    appliedBy  string
    
    // Curse-specific state
    timesTriggered int
    lastTriggered  time.Time
    
    // Event subscriptions
    subscriptions []string
}

func (c *CurseCondition) GetID() string   { return c.id }
func (c *CurseCondition) GetType() string { return "condition.curse" }

func (c *CurseCondition) Apply(bus events.EventBus) error {
    // Subscribe to relevant events based on curse type
    // Track subscriptions for cleanup
    return nil
}
```

## Alternatives Considered

### Pure Event Handlers
- ✅ Simpler implementation
- ❌ No persistence
- ❌ No identity or state
- ❌ Can't query conditions

### Separate Domain Objects
- ✅ Clean separation from entities
- ❌ Need parallel storage system
- ❌ Inconsistent with entity model
- ❌ More complexity for little benefit

## References

- Issue #19: Implement conditions package
- ADR-0002: Hybrid Event-Driven Architecture
- Journey: Architectural Dragons (storage patterns)