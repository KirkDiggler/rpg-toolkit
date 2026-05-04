# Event Participant Ecosystem

*Journey Entry: 2025-01-03*

## The Realization

While designing the conditions system, we realized our event bus isn't just for conditions - it's the nervous system of the entire game. Many different types of game concepts will participate in the event system in different ways.

## Types of Event Participants

### 1. Conditions (Status Effects)
- **Examples**: Blessed, Poisoned, Cursed, Diseased
- **Behavior**: React to events, modify them
- **Lifetime**: Temporary or permanent, persist across sessions
- **Identity**: Yes - each instance needs unique ID

### 2. Features (Character Abilities)
- **Examples**: Sneak Attack, Rage, Divine Smite
- **Behavior**: React to events, modify them
- **Lifetime**: Permanent parts of a character
- **Identity**: Maybe - might be identified by type + owner

### 3. Proficiencies
- **Examples**: Weapon proficiencies, Skill expertise
- **Behavior**: Checked by handlers, don't actively participate
- **Lifetime**: Permanent character data
- **Identity**: No - just data

### 4. Active Spell Effects
- **Examples**: Telekinesis, Spiritual Weapon, Wall of Fire
- **Behavior**: React to events AND player commands
- **Lifetime**: Concentration or duration-based
- **Identity**: Yes - need to track/target them

### 5. Equipment Effects
- **Examples**: Flaming sword, Ring of Protection
- **Behavior**: Active when equipped/attuned
- **Lifetime**: While equipped
- **Identity**: Via the item entity

### 6. Environmental Effects
- **Examples**: Difficult terrain, Darkness, Antimagic field
- **Behavior**: Affect events in an area
- **Lifetime**: Scene/encounter based
- **Identity**: Maybe via location entity

## Patterns Emerging

### The "Activatable" Pattern
Some entities need to register/unregister event handlers:
```go
type Activatable interface {
    Activate(bus events.EventBus) error   // Register handlers
    Deactivate(bus events.EventBus) error // Cleanup handlers
}
```

### The "Commanded" Pattern
Some effects respond to player commands:
```go
type Commanded interface {
    ExecuteCommand(cmd Command) error
}
```

### The "Conditional" Pattern
Many things only apply under certain conditions:
```go
// In handler
if !isRaging(character) { return nil }
if !hasProficiency(character, weapon) { return nil }
if !inDarkness(location) { return nil }
```

## Open Questions

1. **Subscription Management**: Who tracks what subscriptions belong to what entity?

2. **Activation Timing**: When do permanent features register their handlers?
   - On character load?
   - On session start?
   - Always active?

3. **Performance**: With many participants, how do we keep event handling fast?
   - Indexed lookups?
   - Smart routing?
   - Lazy loading?

4. **Testing**: How do we test complex interactions?
   - Mock bus?
   - Test harness?
   - Scenario builders?

5. **Debugging**: How do we trace what modified an event?
   - Event history?
   - Handler ordering visualization?
   - Debug mode?

## Interface Evolution Thoughts

The current `core.Entity` interface is minimal:
```go
type Entity interface {
    GetID() string
    GetType() string
}
```

As we add more participant types, we might need:
- Better method names (EntityID()? Identifier()?)
- Optional interfaces for capabilities
- Metadata for runtime inspection

## Next Steps

1. Implement conditions with this entity-based approach
2. Create a simple feature to validate the pattern
3. Build tools for debugging event flow
4. Consider a registration system for participant types

## Lesson Learned

Starting with "what is a condition?" led us to realize we're building a general-purpose event participation system. Many game concepts will use this same pattern. This is the power of the hybrid architecture - it's not just for combat modifiers, it's for the entire game engine.