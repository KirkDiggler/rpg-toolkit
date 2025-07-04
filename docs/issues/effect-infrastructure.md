# Issue: Create Shared Effect Infrastructure

## Background

While implementing proficiencies (#29), we discovered that conditions and proficiencies share nearly identical patterns:
- Subscription management
- Apply/Remove lifecycle
- Active/inactive state tracking
- Event handler cleanup

This pattern will likely repeat for resources (#30), features (#33), equipment effects (#31), and other game mechanics.

## Goal

Create a shared infrastructure package that provides common functionality for all effect-based game mechanics while maintaining domain clarity.

## Requirements

### 1. Core Effect Infrastructure
- Base type with subscription management
- Lifecycle methods (Apply/Remove)
- State tracking (active/inactive)
- Automatic cleanup of event subscriptions

### 2. Behavioral Interfaces
Effects should be composable from behaviors:
- **ConditionalEffect**: Only applies under certain conditions
- **ResourceConsumer**: Consumes limited resources when used
- **TemporaryEffect**: Has duration/expiration
- **StackableEffect**: Rules for multiple instances
- **PropertyModifier**: Modifies entity properties (STR, inventory slots, etc.)
- **DiceModifier**: Adds dice expressions to rolls

### 3. Domain Type Integration
- Conditions, proficiencies, etc. remain distinct types
- They compose/embed the shared infrastructure
- No loss of domain clarity

## Proposed Structure

```
mechanics/effects/
├── core.go              // EffectCore base type
├── tracker.go           // SubscriptionTracker
├── conditional.go       // ConditionalEffect interface
├── resource.go          // ResourceConsumer interface
├── temporary.go         // TemporaryEffect interface
├── stackable.go         // StackableEffect interface
├── property.go          // PropertyModifier interface
└── dice_modifier.go     // DiceModifier interface
```

## Usage Example

```go
// Domain type composes effect infrastructure
type SimpleProficiency struct {
    effects.Core          // Embedded base functionality
    owner   core.Entity
    subject string
    
    // Optional behaviors
    conditional effects.ConditionalEffect
}

// Core handles subscription management
func (p *SimpleProficiency) Apply(bus EventBus) error {
    // Check condition if present
    if p.conditional != nil && !p.conditional.CheckCondition(ctx) {
        return ErrConditionNotMet
    }
    
    // Use embedded Core for lifecycle
    return p.Core.Apply(bus)
}
```

## Benefits

1. **Eliminate Code Duplication**: ~72 lines of duplicate code removed
2. **Consistent Patterns**: All effects work the same way
3. **Composable Behaviors**: Mix and match as needed
4. **Maintainable**: Fix bugs in one place
5. **Extensible**: Add new behaviors without changing existing code
6. **Domain Clarity**: Proficiency is still Proficiency, not generic Effect

## Success Criteria

1. Extract common code from SimpleCondition and SimpleProficiency
2. Both existing types work unchanged (just using shared infrastructure)
3. Clear documentation and examples
4. Unit tests for all behaviors
5. No loss of functionality or domain clarity

## Implementation Order

1. Create mechanics/effects module structure
2. Implement EffectCore and SubscriptionTracker
3. Migrate SimpleProficiency to use EffectCore
4. Migrate SimpleCondition to use EffectCore
5. Add behavioral interfaces as needed
6. Use infrastructure for new mechanics (resources, features)

## Related

- ADR-0005: Effect Composition Pattern
- Journey 005: Effect Composition Discovery
- Issue #29: Proficiency System (revealed the pattern)
- Issue #30-33: Future systems that will use this