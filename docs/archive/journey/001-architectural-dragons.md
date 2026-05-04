# Architectural Dragons 🐉

Date: 2025-01-03

## Overview

As we build out the event-driven RPG toolkit, we're discovering architectural questions that need careful thought. This document captures these "dragons" - challenges that could bite us if we don't handle them thoughtfully.

## Current Dragons

### 1. Event Context: Typed vs Generic

**The Question**: Should Event Context have typed getters like `GetAttackType()` or stay generic with `Get("attack_type")`?

**Trade-offs**:
- Typed: Better IDE support, compile-time safety, but couples events to specific use cases
- Generic: Maximum flexibility, but runtime errors and string constants everywhere

**Current Thinking**: Stay generic for now, but maybe provide typed wrapper helpers in game-specific modules?

### 2. Storage: The Big Missing Piece

**The Question**: Where does game state live? Who owns the truth?

**Challenges**:
- Events are transient - where do results go?
- How do we query current state (HP, conditions, etc.)?
- Do we go event-sourced? Traditional CRUD? Hybrid?

**Current Thinking**: We've been glossing over this. Need storage interfaces before we go much further.

### 3. Advantage/Disadvantage: RPG-Specific Mechanics

**The Question**: How do we handle mechanics that work differently across game systems?

**Examples**:
- D&D 5e: Roll 2d20, take highest/lowest
- Other systems: Add/subtract dice, reroll failures, etc.

**Current Thinking**: These need to be injected from rulebook modules. The core should provide hooks, not implementations.

### 4. Dice Notation Parsing

**The Question**: Should we support "2d6+3" string parsing?

**Trade-offs**:
- Convenient for users and config files
- Error handling gets complex
- Do we support full expressions? "2d6+1d4+3"?

**Current Thinking**: Make it an optional add-on module. Core uses programmatic API.

## Emerging Patterns

### The Shallow Slice Test

To validate our architecture, we need:
1. A simple entity (monster) that can receive conditions
2. Event publishers (attack, damage, etc.)
3. Event subscribers (conditions modifying rolls)
4. See the full flow work end-to-end

### Mockability Requirements

Everything that's random or external needs interfaces:
- Dice roller (for deterministic tests)
- Time (for duration tests)
- Storage (for unit tests)

## Open Questions

1. **Event Ordering**: If multiple handlers modify the same thing, who wins?
2. **Event Replay**: Should we support replaying events for debugging?
3. **Performance**: How many subscribers before we need optimization?
4. **Error Handling**: What happens when a handler fails?

## Next Steps

1. Implement dice with mockable roller
2. Create minimal entity + condition example
3. Identify storage interface needs
4. Document rulebook injection points

## Lessons Learned

- Our event bus breakthrough is just the beginning
- Every decision has ripple effects
- Better to identify dragons early than fight them later
- The journey is as valuable as the destination

---

*"Here be dragons" - Ancient cartographer wisdom*