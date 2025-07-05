# Event System Enhancement Plan

## Overview
This document outlines the enhancements needed to bring the toolkit's event system to feature parity with the Discord bot's implementation.

## 1. Duration System (Priority: Critical)

### New Types to Add:
```go
// Duration represents how long something lasts
type Duration interface {
    Type() DurationType
    IsExpired(currentTime time.Time, currentRound int) bool
    Description() string
}

// DurationType enum
type DurationType int

const (
    DurationTypePermanent DurationType = iota
    DurationTypeRounds
    DurationTypeMinutes
    DurationTypeHours
    DurationTypeEncounter
    DurationTypeConcentration
    DurationTypeShortRest
    DurationTypeLongRest
    DurationTypeUntilDamaged
    DurationTypeUntilSave
)

// Concrete implementations
type PermanentDuration struct{}
type RoundsDuration struct{ Rounds int }
type MinutesDuration struct{ Minutes int }
type HoursDuration struct{ Hours int }
type EncounterDuration struct{}
type ConcentrationDuration struct{}
type ShortRestDuration struct{}
type LongRestDuration struct{}
type UntilDamagedDuration struct{}
type UntilSaveDuration struct{ 
    Ability string
    DC int 
}
```

## 2. Type-Safe Event Types (Priority: High)

### Changes to event.go:
```go
// Add EventType as int type
type EventType int

// Convert string constants to typed constants
const (
    EventTypeBeforeAttackRoll EventType = iota
    EventTypeOnAttackRoll
    EventTypeAfterAttackRoll
    // ... etc
)

// Add both string and typed access
type Event interface {
    Type() string         // Keep for backward compatibility
    TypedType() EventType // New typed access
    // ... existing methods
}
```

## 3. Context Keys Constants (Priority: High)

### New file: context_keys.go
```go
package events

// Context keys for common event data
const (
    // Combat keys
    ContextKeyAttacker = "attacker"
    ContextKeyTarget = "target"
    ContextKeyWeapon = "weapon"
    ContextKeySpell = "spell"
    ContextKeyDamageType = "damage_type"
    ContextKeyDamageAmount = "damage_amount"
    
    // Roll keys
    ContextKeyRollResult = "roll_result"
    ContextKeyRollModifiers = "roll_modifiers"
    ContextKeyAdvantage = "advantage"
    ContextKeyDisadvantage = "disadvantage"
    
    // Save/Check keys
    ContextKeyAbility = "ability"
    ContextKeyDC = "dc"
    ContextKeySaveType = "save_type"
    ContextKeySkill = "skill"
    
    // Effect keys
    ContextKeyEffect = "effect"
    ContextKeyCondition = "condition"
    ContextKeyDuration = "duration"
    ContextKeyConcentration = "concentration"
)
```

## 4. Typed Context Accessors (Priority: High)

### Enhance Context interface:
```go
type Context interface {
    // Existing methods
    Get(key string) (interface{}, bool)
    Set(key string, value interface{})
    
    // New typed accessors
    GetInt(key string) (int, bool)
    GetString(key string) (string, bool)
    GetBool(key string) (bool, bool)
    GetEntity(key string) (core.Entity, bool)
    GetDuration(key string) (Duration, bool)
    
    // Existing modifier methods
    AddModifier(modifier Modifier)
    Modifiers() []Modifier
}
```

## 5. Enhanced Modifier System (Priority: High)

### Update Modifier interface:
```go
type Modifier interface {
    // Existing methods
    Source() string
    Type() string
    Value() interface{}
    ModifierValue() ModifierValue
    Priority() int
    
    // New methods
    Condition(event Event) bool    // Check if modifier applies
    Duration() Duration            // How long the modifier lasts
    SourceDetails() ModifierSource // Rich source information
}

// New type for rich source information
type ModifierSource struct {
    Type        string // "spell", "condition", "feature", etc.
    Name        string // "Bless", "Rage", etc.
    Description string // Human-readable description
    Entity      core.Entity // Who/what created it
}
```

## 6. Event Builder Pattern (Priority: Medium)

### Add builder methods to GameEvent:
```go
// Builder methods for fluent API
func (e *GameEvent) WithSource(source core.Entity) *GameEvent {
    e.source = source
    return e
}

func (e *GameEvent) WithTarget(target core.Entity) *GameEvent {
    e.target = target
    return e
}

func (e *GameEvent) WithContext(key string, value interface{}) *GameEvent {
    e.context.Set(key, value)
    return e
}

func (e *GameEvent) WithModifier(modifier Modifier) *GameEvent {
    e.context.AddModifier(modifier)
    return e
}
```

## 7. Store Modifiers on Events (Priority: Medium)

### Update GameEvent struct:
```go
type GameEvent struct {
    eventType string
    source    core.Entity
    target    core.Entity
    timestamp time.Time
    context   Context
    cancelled bool
    
    // New field
    eventModifiers []Modifier // Modifiers that come with the event
}
```

## Implementation Order

1. **Phase 1: Foundation**
   - Duration system (new file: duration.go)
   - Context keys constants (new file: context_keys.go)
   - Typed context accessors (update event.go)

2. **Phase 2: Type Safety**
   - Type-safe event types (update event.go)
   - Enhanced modifier system (update event.go)

3. **Phase 3: Convenience**
   - Event builder pattern (update event.go)
   - Store modifiers on events (update event.go)

## Migration Strategy

1. All changes should be backward compatible
2. String-based event types remain for compatibility
3. New features are additions, not replacements
4. Provide migration guide for Discord bot

## Testing Requirements

1. Unit tests for all new duration types
2. Tests for typed context accessors
3. Tests for conditional modifiers
4. Integration tests with existing systems
5. Performance benchmarks