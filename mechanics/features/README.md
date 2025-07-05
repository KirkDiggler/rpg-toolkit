# Features Module

The features module provides a comprehensive system for implementing character abilities, racial traits, and feats in tabletop RPGs.

## Overview

Features represent special abilities that characters can have:
- **Racial Features**: Abilities from a character's race (e.g., Darkvision)
- **Class Features**: Abilities from a character's class (e.g., Rage, Sneak Attack)
- **Subclass Features**: Specialized abilities from subclasses
- **Feats**: Optional abilities characters can learn
- **Item Features**: Abilities granted by magical items

## Feature Types

### By Timing

Features are categorized by when they take effect:

1. **Passive**: Always active (e.g., Darkvision)
2. **Triggered**: React to game events (e.g., Sneak Attack)
3. **Activated**: Must be activated by the player (e.g., Rage)

## Usage

### Creating a Feature

```go
// Passive feature
darkvision := features.NewBasicFeature("darkvision", "Darkvision").
    WithDescription("You can see in darkness").
    WithType(features.FeatureRacial).
    WithTiming(features.TimingPassive).
    WithModifiers(visionModifier)

// Activated feature with resources
rage := features.NewBasicFeature("rage", "Rage").
    WithType(features.FeatureClass).
    WithTiming(features.TimingActivated).
    WithResources(rageUsesResource).
    WithEventListeners(RageListener{})

// Triggered feature
sneakAttack := features.NewBasicFeature("sneak_attack", "Sneak Attack").
    WithType(features.FeatureClass).
    WithTiming(features.TimingTriggered).
    WithEventListeners(SneakAttackListener{})
```

### Managing Features

The feature system uses a hybrid approach:

1. **FeatureRegistry** - For feature definitions and discovery
2. **FeatureHolder** - Entities store and manage their own features

```go
// Registry for feature definitions
registry := features.NewRegistry()
registry.RegisterFeature(rageFeature)
registry.RegisterFeature(sneakAttackFeature)

// Entities implement FeatureHolder
type Character struct {
    features.FeatureHolder
    // other fields...
}

// Add features to entities
character.AddFeature(rageFeature)

// Activate features
character.ActivateFeature("rage", eventBus)

// Query available features
available := registry.GetFeaturesForClass("barbarian", 5)
```

### Event Integration

Features can listen to and modify game events:

```go
type RageListener struct{}

func (r RageListener) EventTypes() []string {
    return []string{
        events.EventOnDamageRoll,
        events.EventBeforeTakeDamage,
    }
}

func (r RageListener) HandleEvent(feature Feature, entity Entity, event Event) error {
    switch event.Type() {
    case events.EventOnDamageRoll:
        // Add rage damage bonus
        event.Context().AddModifier(rageDamageModifier)
    case events.EventBeforeTakeDamage:
        // Add damage resistance
        event.Context().AddModifier(rageResistanceModifier)
    }
    return nil
}
```

## Examples

See the `examples` directory for complete implementations of:
- **Rage**: Barbarian's signature ability with damage bonus and resistance
- **Sneak Attack**: Rogue's precision damage ability
- **Darkvision**: Racial ability to see in darkness

## Integration with Other Modules

- **Events**: Features use event listeners to react to game events
- **Resources**: Features can provide or consume resources
- **Proficiency**: Features can grant proficiencies
- **Conditions**: Features can apply or react to conditions

## Design Philosophy

The feature system follows these principles:

1. **Flexibility**: Support any game system, not just D&D
2. **Composition**: Features can combine multiple behaviors
3. **Event-Driven**: Features interact through the event system
4. **Type Safety**: Use Go's type system for compile-time checks