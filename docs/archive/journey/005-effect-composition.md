# Journey 005: Effect Composition Pattern

## The Discovery

While implementing the proficiency system (#29), we noticed striking similarities with the existing conditions system. Both had nearly identical:
- Subscription management code
- Apply/Remove lifecycle
- Active/inactive state tracking
- Event handler cleanup

This led to a deeper realization: proficiencies, conditions, resources, features, and equipment effects are all variations of the same pattern - **game modifiers that affect behavior through the event system**.

## Initial Instinct: Extract Base Class

Our first thought was to create a shared `Effect` base class that both conditions and proficiencies could inherit from. This would eliminate ~72 lines of duplicate code.

However, analyzing this approach revealed problems:
- Loss of domain clarity (everything becomes an "Effect")
- Forced generalization of terminology
- Only 2 implementations - too early to abstract
- Minimal code savings for the complexity added

## The Breakthrough: Composition Over Inheritance

The key insight came from recognizing that effects have different **behaviors** that can be mixed and matched:

1. **Conditional** - Only apply under certain circumstances
2. **Resource Consuming** - Use limited resources
3. **Temporary** - Have duration/expiration
4. **Stackable** - Rules for multiple instances
5. **Targeted** - Affect specific entities

Instead of forcing all effects into one hierarchy, we can compose them from behavioral building blocks.

## Examples That Clarified the Pattern

- **Weapon Proficiency**: Core effect + Conditional (only with that weapon)
- **Barbarian Rage**: Core effect + Conditional + Resource + Duration
- **Bless Spell**: Core effect + Duration + Non-stackable
- **Poison**: Core effect + Duration + Stackable + Saving throw

Each game mechanic keeps its domain identity while sharing common infrastructure.

## The Event Bus as Enabler

The event bus is what makes this composition powerful. Effects aren't just data - they're active participants that can:
- React to any game event
- Modify other effects' behaviors
- Chain complex interactions
- Be discovered and queried

## Benefits Realized

1. **Domain Clarity**: A Proficiency is still a Proficiency, not a generic Effect
2. **Flexible Composition**: New effect types are just new combinations
3. **Reusable Behaviors**: Write conditional logic once, use everywhere
4. **Testable Components**: Each behavior can be tested in isolation
5. **Extensible**: New behaviors can be added without changing existing code

## Implementation Strategy

Rather than a big refactor:
1. Build the effect core and behaviors alongside existing code
2. Gradually migrate conditions and proficiencies
3. Use the new patterns for resources and features from the start
4. Refactor only when the benefits are clear

## Important Realization: Dynamic Dice Modifiers

While designing the effect system, we discovered a critical requirement: dice modifiers must be rolled dynamically, not pre-calculated. 

For example, Bless adds "1d4" to attack rolls - this needs to be a fresh d4 roll for each attack, not a fixed +3 rolled when Bless is applied. This means our effect system needs a `DiceModifier` behavior that can express and handle dice expressions.

## Lessons Learned

- **Premature abstraction is dangerous** - We almost created unnecessary complexity
- **Composition > Inheritance** - Especially in Go
- **Domain modeling matters** - Keep game concepts clear
- **The event bus is a superpower** - It enables loose coupling with deep integration
- **Journey documents help** - Writing this clarified our thinking
- **Question assumptions** - Dice modifiers aren't numbers, they're expressions

## Next Steps

1. Create ADR for effect composition pattern
2. Implement core effect behaviors
3. Build resources using the new pattern
4. Gradually migrate existing systems