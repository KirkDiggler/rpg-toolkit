# ADR-0001: Modifier Value Interface Design

Date: 2025-01-03

## Status

Proposed

## Context

The RPG toolkit needs a consistent way to handle modifiers that can be either:
- Flat values (e.g., +2 proficiency bonus)
- Dice rolls (e.g., +1d4 from Bless)
- Multipliers (e.g., 0.5x for resistance)
- Special modifiers (e.g., advantage/disadvantage)

These modifiers need to:
1. Pass through the event system
2. Display clearly in combat logs
3. Be rolled at creation time (not deferred)
4. Avoid ugly type assertions

## Decision

We will update the `Modifier` interface to return a typed `ModifierValue` instead of `interface{}`:

```go
type Modifier interface {
    Source() string
    Type() string
    Priority() int
    ModifierValue() ModifierValue
}

type ModifierValue interface {
    GetValue() int
    GetDescription() string
}
```

All modifier types will implement `ModifierValue`:
- `RawValue` - for flat numeric modifiers
- `DiceValue` - for dice rolls (rolled at creation)
- `Multiplier` - for scaling modifiers
- `Advantage/Disadvantage` - for special roll mechanics

## Consequences

### Positive
- Type-safe modifier processing without type assertions
- Clear separation between value and description
- Dice are rolled when modifiers are created (deterministic)
- Clean, idiomatic Go code
- Consistent interface for all modifier types

### Negative
- Slightly more verbose than raw `interface{}`
- Requires implementing two methods for each modifier type

### Neutral
- Modifiers become immutable once created
- Event system carries concrete values, not potential rolls

## Example

```go
// Creating a dice modifier (rolls immediately)
blessBonus := NewDiceValue(1, 4, "bless")
// Result: DiceValue{notation: "d4", rolls: [3], total: 3, source: "bless"}

// Clean processing
for _, mod := range modifiers {
    mv := mod.ModifierValue()
    total += mv.GetValue()
    descriptions = append(descriptions, mv.GetDescription())
}

// Output: "Attack: d20[15] + 2 (proficiency) + d4[3]=3 (bless) = 20"
```