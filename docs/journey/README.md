# The Journey

This directory contains the ongoing architectural journey of the RPG Toolkit. Unlike ADRs (Architecture Decision Records) which document final decisions, these documents capture:

- Questions we're grappling with
- Design explorations
- "Dragons" we've identified but not yet slain
- Lessons learned along the way

## Why Document the Journey?

1. **For Future Contributors**: Understanding why things are the way they are
2. **For Ourselves**: Remembering what we've already considered
3. **For the Community**: Open source is as much about the process as the result

## Current Documents

- [001: Architectural Dragons](001-architectural-dragons.md) - Big challenges we see ahead
- [002: Dice Package Design](002-dice-package-design.md) - Initial dice package design
- [003: Event Participant Ecosystem](003-event-participant-ecosystem.md) - Event system architecture
- [004: Conditions System](004-conditions-system.md) - Status effects and relationships
- [005: Effect Composition](005-effect-composition.md) - Designing composable effects system
- [006: Feature Management Pattern](006-feature-management-pattern.md) - Exploring feature system patterns
- [007: Enhanced Conditions](007-enhanced-conditions.md) - Advanced condition system capabilities
- [008: Dice Roller Design](008-dice-roller-design.md) - Global vs instance-based dice patterns
- [009: Generics in Conditions](009-generics-in-conditions.md) - Type safety vs flexibility trade-offs
- [010: Generic Event Patterns](010-generic-event-patterns.md) - Event-driven restoration triggers
- [011: Spell System Design](011-spell-system-design.md) - Designing the spell casting system

## Document Types

- **Dragons** 🐉: Architectural challenges and unknowns
- **Designs** 📐: Working through implementation approaches
- **Lessons** 💡: What we learned after implementing something
- **Questions** ❓: Open questions we're still pondering

## Technical Examples

The `/examples` directory contains working implementations that prove the architectural concepts explored in these journey documents. These examples demonstrate different perspectives and integration approaches:

### Core Architecture Demonstrations
- **[Simple Combat](/examples/simple_combat/)** - Shows event-driven architecture flow with attack/damage calculations, modifier system, and handler priorities
- **[Conditions Demo](/examples/conditions_demo/)** - Demonstrates relationship management, concentration mechanics, and aura effects integration

### D&D 5e Implementation Examples
- **[Proficiency System](/examples/dnd5e/proficiency/)** - Complete D&D 5e proficiency implementation using event-driven bonuses
- **[Spells & Conditions](/examples/dnd5e/conditions/)** - D&D 5e specific condition implementations and spell interactions

### Migration & Integration Guides
- **[DND Bot Integration](/examples/dndbot_integration/)** - Practical migration strategy for existing Discord bot systems
- **[Event Bus Migration](/examples/dndbot_integration/event_bus_migration_guide.md)** - Step-by-step event system replacement guide

These examples serve as:
1. **Proof of Concept**: Demonstrating that the architectural decisions work in practice
2. **Migration References**: Showing how to integrate toolkit into existing systems
3. **Implementation Patterns**: Providing templates for common RPG mechanics
4. **Testing Ground**: Real code that validates the design decisions documented in these journey entries

## Contributing

If you're exploring a design or wrestling with an architectural question, add a document here! Number it sequentially and give it a descriptive name.

Remember: This is about the journey, not the destination. It's okay to document ideas that don't work out - that's valuable learning too!