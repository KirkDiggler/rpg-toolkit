# ADR-0012: Selectables Tool Architecture

## Status
Proposed

## Context

The RPG Toolkit needs a universal weighted random selection system that can work with any content type. This "selectables" or "grabbag" tool would enable procedural content generation across multiple domains:

- Monster selection by challenge rating and environment
- Treasure generation with rarity tiers and level-appropriate loot
- Encounter activities, plot hooks, and environmental effects
- Generic weighted random choices for any game content

Current gap: No standardized way to perform weighted random selection with support for:
- Generic type safety
- Nested/hierarchical selection tables
- Context-based selection modifications
- Multiple selection modes (single, multiple, unique)
- Integration with existing dice systems

## Decision

We will implement a generic selectables tool as `tools/selectables` that provides:

### Core Architecture

1. **Generic Selection Interface**
   ```go
   type SelectionTable[T any] interface {
       Add(item T, weight int) SelectionTable[T]
       AddTable(name string, table SelectionTable[T], weight int) SelectionTable[T]
       Select(ctx context.Context) (T, error)
       SelectMany(ctx context.Context, count int) ([]T, error)
       SelectUnique(ctx context.Context, count int) ([]T, error)
   }
   ```

2. **Selection Context**
   ```go
   type SelectionContext struct {
       Conditions map[string]interface{}
       Modifiers  []SelectionModifier
       Dice       dice.Roller
   }
   ```

3. **Hierarchical Support**
   - Nested tables for category-then-item selection
   - Weighted table references within tables
   - Dynamic weight modification based on context

### Implementation Types

1. **BasicTable[T]**: Simple weighted random selection
2. **ConditionalTable[T]**: Context-aware selection with conditional weights
3. **HierarchicalTable[T]**: Nested category-based selection
4. **CompositeTable[T]**: Multiple selection strategies combined

### Integration Points

- **Dice Package**: Use existing dice.Roller for randomization
- **Events Package**: Selection events for debugging/analytics
- **Context Package**: Standard Go context for cancellation/deadlines

### Selection Modes

1. **Single Selection**: `Select()` - one item with replacement
2. **Multiple Selection**: `SelectMany()` - multiple items with replacement
3. **Unique Selection**: `SelectUnique()` - multiple items without replacement

## Rationale

### Why Generic Types?
- Type safety at compile time
- Reusable across all content types (monsters, items, encounters, etc.)
- Clear API contracts and better IDE support

### Why Hierarchical Tables?
- Real-world use cases often need category-then-item selection
- Enables complex procedural generation patterns
- Supports nested probability distributions

### Why Context-Based Selection?
- Game state affects selection probability (player level, environment, etc.)
- Conditional selection based on game rules
- Dynamic weight modification without table reconstruction

### Why Multiple Selection Modes?
- Different use cases need different selection behaviors
- Unique selection prevents duplicate results when inappropriate
- Flexible API supports various procedural generation needs

## Consequences

### Positive
- Universal tool for all weighted random selection needs
- Type-safe and reusable across the entire toolkit
- Supports complex procedural generation patterns
- Integrates cleanly with existing dice and events systems
- Follows established RPG Toolkit patterns

### Negative
- Additional complexity in the tools layer
- Generic interface may have learning curve
- Need to maintain backwards compatibility as use cases evolve

### Neutral
- New module increases toolkit surface area
- Will need comprehensive documentation and examples

## Implementation Plan

### Phase 1: Core Foundation
1. Define generic interfaces and basic types
2. Implement BasicTable[T] with weighted selection
3. Integration with dice package
4. Basic test coverage

### Phase 2: Advanced Features
1. Conditional selection with context
2. Hierarchical/nested table support
3. Multiple selection modes
4. Events integration

### Phase 3: Optimization & Polish
1. Performance optimization for large tables
2. Memory efficiency improvements
3. Comprehensive examples and documentation
4. Integration examples with other modules

## Example Usage

```go
// Monster selection by challenge rating
monsterTable := selectables.NewBasicTable[Monster]().
    Add(goblin, 50).
    Add(orc, 30).
    Add(troll, 10).
    Add(dragon, 1)

// Treasure with rarity tiers
treasureTable := selectables.NewHierarchicalTable[Item]().
    AddTable("common", commonItems, 70).
    AddTable("uncommon", uncommonItems, 25).
    AddTable("rare", rareItems, 5)

// Context-based selection
ctx := selectables.NewContext().
    Set("player_level", 5).
    Set("environment", "forest")

selectedMonster, err := monsterTable.Select(ctx)
```

## Related ADRs
- ADR-0002: Hybrid Architecture (tools layer placement)
- ADR-0011: Environment Generation (procedural content integration)

## References
- GitHub Issue #59: Selectables Tool Requirements
- RPG Toolkit Architecture Guidelines
- Go Generics Best Practices