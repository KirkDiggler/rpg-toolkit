# Journey 005: Generics in the Conditions System

## Date: 2025-01-03

## The Observation

While implementing the conditions system, we created a `RelationshipManager` that handles different relationship types through a type constant and metadata map:

```go
type Relationship struct {
    Type       RelationshipType
    Source     core.Entity
    Conditions []Condition
    Metadata   map[string]any   // Holds type-specific data
}
```

This works but has some code smells:
- Type-specific logic scattered through switch statements
- Metadata is untyped (`map[string]any`)
- No compile-time safety for relationship-specific operations

## Where Generics Could Help

### Current Approach
```go
// Creating an aura requires specific metadata
err := rm.CreateRelationship(
    RelationshipAura,
    source,
    conditions,
    map[string]any{"range": 10}, // Hope we remember this!
)
```

### Potential Generic Approach
```go
// Type-safe aura relationship
type AuraRelationship[T Condition] struct {
    Source     core.Entity
    Conditions []T
    Range      int  // Compile-time type safety!
}

// Clear, type-safe API
aura := NewAuraRelationship(source, conditions, 10)
rm.Add(aura)
```

## Benefits of Generics Here

1. **Type Safety**: Each relationship type could have its own struct with proper fields
2. **Better IDE Support**: Autocomplete would show exactly what each relationship needs
3. **Clearer APIs**: No more guessing what goes in the metadata map
4. **Easier Testing**: Can test each relationship type independently

## Why We're Not Doing It Now

1. **Working Code**: Current implementation works and can be used in the Discord bot
2. **Learning Curve**: Want to see real usage patterns first
3. **Refactoring Scope**: Would touch every relationship type
4. **YAGNI**: Might not need all the relationship types we defined

## Questions to Answer Through Usage

1. Which relationship types actually get used?
2. Do we need all six types or just 2-3?
3. What patterns emerge in real game systems?
4. How complex does the metadata get?

## Potential Migration Path

If we decide to use generics later:

1. Create generic relationship types alongside existing ones
2. Deprecate old `CreateRelationship` method
3. Migrate usage gradually
4. Remove old system once migrated

## Decision Point

After using the conditions system in the Discord bot for a few sessions, we should revisit this and decide:
- Keep the current flexible but less type-safe approach
- Refactor to use generics for better type safety
- Simplify to fewer relationship types first, then consider generics

## Related Considerations

- Event system might also benefit from generics
- Could type event payloads instead of using Context
- But same principle applies: get it working first, optimize later