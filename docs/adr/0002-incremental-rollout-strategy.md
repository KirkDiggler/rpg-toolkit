# ADR-0002: Incremental Rollout Strategy for Modifier System

Date: 2025-01-03

## Status

Proposed

## Context

We have a solid design for the modifier system (ADR-0001), but need to implement it incrementally to:
- Keep PRs focused and reviewable
- Test each piece thoroughly
- Avoid over-engineering before we validate the design
- Show clear examples of how the system works

## Decision

Roll out the modifier system in focused PRs:

### PR 1: Core Modifier Interface
- Add `ModifierValue` interface to events package
- Update `Modifier` interface to use `ModifierValue`
- Implement basic types: `RawValue`, `DiceValue`
- Update `ProcessModifiers` to use new interface

### PR 2: Dice Package Integration  
- Update dice package modifiers to implement `ModifierValue`
- Remove the current `Apply()` method from dice types
- Add clear examples in dice README

### PR 3: Event Flow Example
- Create `examples/attack_roll/` showing full flow:
  - Publisher adds modifiers (proficiency, ability, bless)
  - Event processor collects and applies modifiers
  - Clear output showing how values combine
- Update events README with usage patterns

### PR 4: Conditions Package (Later)
- Only after modifier system is proven
- Start with simple conditions (no duration management)
- Focus on event subscription pattern

## Consequences

### Positive
- Each PR has a single, clear purpose
- Can validate design at each step
- Examples prove the system works
- Team can review incrementally

### Negative
- Takes longer to get full system in place
- Some temporary scaffolding might be needed

### Neutral
- Conditions package delayed until foundation is solid
- Focus on demonstrating patterns over full implementation

## Example Structure

```
events/
├── modifier_value.go      # PR 1: Interface definition
├── modifier_types.go      # PR 1: RawValue, DiceValue
├── modifier_processor.go  # PR 1: Processing logic
└── README.md             # PR 3: Usage examples

dice/
├── modifier.go           # PR 2: Updated implementations
└── README.md            # PR 2: Dice modifier examples

examples/
└── attack_roll/         # PR 3: Full working example
    ├── main.go
    └── README.md
```