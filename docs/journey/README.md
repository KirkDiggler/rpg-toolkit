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

## The Journey (Chronological)

*Note: Along the journey, we discovered we weren't great at numbering. The filenames preserve our original (sometimes duplicate) numbering, but here's the actual chronological order:*

1. [Architectural Dragons](001-architectural-dragons.md) - Big challenges we see ahead
2. [Dice Package Design](002-dice-package-design.md) - Initial dice package design
3. [Event Participant Ecosystem](003-event-participant-ecosystem.md) - Event system architecture
4. [Conditions System](004-conditions-system.md) - Status effects and relationships
5. [Effect Composition](005-effect-composition.md) - Designing composable effects system
6. [Feature Management Pattern](006-feature-management-pattern.md) - Exploring feature system patterns
7. [Enhanced Conditions](007-enhanced-conditions.md) - Advanced condition system capabilities
8. [Dice Roller Design](008-dice-roller-design.md) - Global vs instance-based dice patterns
9. [Generics in Conditions](009-generics-in-conditions.md) - Type safety vs flexibility trade-offs
10. [Generic Event Patterns](010-generic-event-patterns.md) - Event-driven restoration triggers
11. [Spell System Design](011-spell-system-design.md) - Designing the spell casting system
12. [Spatial Module Design](012-spatial-module-design.md) - Grid and positioning systems
13. [Room Orchestrator Vision](013-room-orchestrator-vision.md) - Multi-room management
14. [Factory vs Helper Patterns](014-factory-vs-helper-patterns.md) - Construction patterns
15. [Environment Generation Design Evolution](015-environment-generation-design-evolution.md) - Procedural generation
16. [AI Assistant Code Quality Lessons](016-ai-assistant-code-quality-lessons.md) - AI collaboration learnings
17. [Encounter System Design](017-encounter-system-design.md) - Encounter mechanics
18. [Content Architecture Breakthrough](018-content-architecture-breakthrough.md) - Content organization
19. [Self-Contained Entities](019-self-contained-entities.md) - Entity independence
20. [Extensible Registry System](020_extensible_registry_system.md) - Plugin architecture
21. [Draft System Redesign](007-draft-system-redesign.md) - Reworking draft mechanics
22. [Features Conditions Refactor](021-features-conditions-refactor.md) - Refactoring relationship
23. [Event System Typed Events](022-event-system-typed-events.md) - Type-safe events
24. [Event Bus Modifiers](014-event-bus-modifiers.md) - Event modification patterns
25. [Event-Driven Combat Flow](040-event-driven-combat-flow.md) - Combat via events
26. [Event Bus Generics Exploration](041-event-bus-generics-exploration.md) - Generic patterns
27. [Event Ref Exploration](042-event-ref-exploration.md) - Event referencing patterns
28. [Data-Driven Runtime Architecture](024-data-driven-runtime-architecture.md) - Runtime loading
29. [Spellcasting System Design](023-spellcasting-system-design.md) - Spell mechanics
30. [Actions Effects Architecture](043-actions-effects-architecture.md) - Action system design
31. [Actions Internal Pattern](043-actions-internal-pattern.md) - Internal patterns
32. [Actions Internal Pattern Clean](043-actions-internal-pattern-clean.md) - Refined approach
33. [Tonight's Insights Summary](043-tonights-insights-summary.md) - Action insights
34. [Rulebooks Own Pipelines](027-rulebooks-own-pipelines.md) - Rulebook responsibility
35. [Pipelines All The Way Down](026-pipelines-all-the-way-down.md) - Pipeline philosophy
36. [Complex DND Mechanics Pipeline](025-complex-dnd-mechanics-pipeline.md) - D&D complexity
37. [Effect Modification Pipeline](024-effect-modification-pipeline.md) - Effect processing
38. [Rage Implementation Lessons](023-rage-implementation-lessons.md) - Barbarian rage learnings
39. [Event Bus Evolution](014-event-bus-evolution.md) - Event system improvements
40. [Typed Topics Design](007-typed-topics-design.md) - Type-safe event topics
41. [Events Carry Actions, Not Objects](044-events-carry-actions-not-objects.md) - Event philosophy breakthrough
42. [Circular Dependencies Events Subpackage](045-circular-dependencies-events-subpackage.md) - Solving import cycles
43. [Feature Implementation Workflows](046-feature-implementation-workflows.md) - Patterns for adding class features
44. [Class-Level Grants Architecture](047-class-level-grants-architecture.md) - Self-contained class definitions with level-based grants
45. [Refs Package Design](047-refs-package-design.md) - Pointers vs values for discoverable type-safe references


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