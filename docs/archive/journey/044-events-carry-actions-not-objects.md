# Journey 044: Events Carry Actions, Not Objects

## The Quest

We set out to implement character feature loading, starting with the barbarian's rage. Simple enough - load a feature from JSON, activate it, have it participate in the event system. The rage feature needed to publish events when activated, and conditions needed to subscribe to those events.

## The Initial Approach

Our first instinct was to have the `ConditionAppliedEvent` carry the full `Condition` object:

```go
type ConditionAppliedEvent struct {
    Target    core.Entity
    Condition *conditions.Condition  // The full object with all its data
}
```

This felt natural - when a condition is applied, pass the whole condition around so subscribers have everything they need.

## The Wall We Hit

Circular dependencies. The `dnd5e` package defined events, but those events referenced `conditions.Condition`. Meanwhile, conditions needed to import `dnd5e` to subscribe to those same events. Classic circular import problem.

We tried several escapes:
- Moving event definitions to conditions package (but then features couldn't publish them)
- Duplicating event types (maintenance nightmare we were actively cleaning up)
- Creating a separate events package (but where does it belong in the hierarchy?)

## The Insight

The breakthrough came from stepping back and asking: **What is an event supposed to communicate?**

Events describe **what happened**, not **what exists**. They're messages about actions and changes, not carriers for entire objects.

When rage is activated, the event shouldn't carry a Condition object - it should describe that a raging condition was applied, who it was applied to, and any specific data about that application.

## The Pattern That Emerged

```go
// Events are thin - they describe actions
type ConditionAppliedEvent struct {
    Target core.Entity      // Who received it
    Type   ConditionType    // What type of condition
    Source string           // What caused it
    Data   any              // Specific data for this event
}

// Features publish with specific data
err := topic.Publish(ctx, ConditionAppliedEvent{
    Target: owner,
    Type:   dnd5e.ConditionRaging,
    Source: r.id,
    Data: RageEventData{
        DamageBonus: 2,
        Level:       5,
    },
})

// Conditions subscribe and type-assert the data they care about
func (r *RagingCondition) onConditionApplied(ctx context.Context, event dnd5e.ConditionAppliedEvent) error {
    if event.Type == dnd5e.ConditionUnconscious && event.Target.GetID() == r.CharacterID {
        // Handle unconscious ending rage
    }
}
```

## The Type + Data Pattern

We landed on a discriminated union pattern that's very Go-like:
- `Type` field acts as a discriminator (strongly typed as `ConditionType`)
- `Data` field holds type-specific data (using `any` with type assertions)
- Type assertions in Go are incredibly fast (1-2ns)
- Publishers know what they're publishing
- Subscribers know what they're looking for

## Why This Matters

1. **Clean boundaries**: Events are just data about what happened. The event package doesn't need to know about the full implementation of conditions or features.

2. **Flexibility**: Different publishers can include different data for the same condition type. A rage from a feature might include level and damage bonus, while a rage from an item might include duration.

3. **The events package as neutral ground**: By moving just the type definitions (`ConditionType`) to the events package, we created a vocabulary that both features and conditions can share without knowing about each other.

4. **Extensibility path**: We can later add a registry pattern for modules to register new condition types, enabling cross-module conditions (imagine a "Homebrew Curses" module adding conditions to base D&D).

## The Turtle Wins Again

Rather than over-engineering for hypothetical future needs, we:
- Solved the immediate problem (feature loading and event publishing)
- Found a pattern that's simple and Go-idiomatic
- Left doors open for future extension without committing to complexity now
- Kept the implementation focused on what we actually need

## Technical Notes

- Bus ownership: Characters own the bus, features receive it through input
- No stored references: Features don't store the bus, they use it and forget it
- Clean lifecycle: Conditions can subscribe/unsubscribe without affecting features
- Type safety through convention: String-based under the hood, but wrapped in types

## The Lesson

Events are not transport mechanisms for objects. They're notifications about what happened. This fundamental insight not only solved our circular dependency but led to a cleaner, more maintainable architecture that better represents the actual flow of information in our system.

Sometimes the best solution comes from questioning what we're really trying to communicate.