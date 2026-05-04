# Journey Document: Encounter System Design

## Date: 2025-07-20

## Context

We're transitioning from a simple encounter system in dnd-bot-discord (attack monster, monster attacks back) to a rich, reactive system with animations in a React app. The goal is to create a comprehensive encounter system that spans multiple projects:
- rpg-toolkit: Core infrastructure
- rpg-api: Game-specific orchestration  
- rpg-dnd5e-web: Rich visualization
- dnd-bot-discord: Companion interface (prototype)

## Journey

### Starting Point

The rpg-toolkit already has:
- Complete spatial system with 2D positioning
- Multi-room orchestration with connections
- Event-driven architecture
- Selectables for weighted random choices
- Environment generation (in progress)

What's missing:
- Entity spawn engine (proposed in ADR-0013)
- AI/behavior infrastructure
- Combat flow management

### Key Insights

1. **The spatial foundation is solid** - We don't need to reinvent positioning, movement, or room management. The spatial module provides everything needed for tactical combat.

2. **Event-driven is the way** - The existing event system perfectly suits AI behaviors. Decisions can be published as events, making the system observable and testable.

3. **Toolkit philosophy holds** - We should provide behavior infrastructure, not implementations. Games decide how monsters think.

### AI/Behavior Design Evolution

#### Initial Thoughts: Simple State Machine
Started with the idea of basic states: idle, attacking, fleeing. But this quickly felt limiting for interesting combat.

#### Consideration: Behavior Trees
More flexible, allows complex behaviors:
- Composites (Sequence, Selector, Parallel)
- Decorators (Repeat, Invert, Timeout)
- Leaves (Actions, Conditions)

But might be overkill for many use cases.

#### Landing: Pluggable Architecture
Why choose? Provide interfaces for multiple paradigms:
1. State machines for simple behaviors
2. Behavior trees for complex AI
3. Utility AI for nuanced decisions

Let the game implementation choose what fits.

### Monster Behavior Templates

Evolved from "monsters attack" to rich templates:
- **Aggressive**: Charge nearest, ignore safety
- **Frightened**: Keep distance, flee when hurt
- **Tactical**: Flank, use terrain, target weak
- **Support**: Heal allies, buff, protect
- **Berserker**: Focus attacker, rage when hurt

These aren't hardcoded - they're weighted decision configurations using Selectables.

### Multi-Project Orchestration

The encounter flow spans projects:

1. **rpg-toolkit** provides:
   - Spawn engine (place entities)
   - Behavior infrastructure
   - Perception/pathfinding
   - Combat events

2. **rpg-api** orchestrates:
   - Turn order/initiative
   - Action resolution
   - State persistence
   - Monster template loading

3. **Web app** visualizes:
   - Room rendering
   - Movement animations
   - Spell effects
   - Health/status display

4. **Discord bot** simplifies:
   - Text summaries
   - Quick commands
   - Static battle maps

### Technical Decisions

1. **Pathfinding placement** - Should live in toolkit as infrastructure, not API
2. **Behavior data format** - JSON/YAML templates over hardcoded behaviors
3. **Animation timing** - Server authoritative, client predicts
4. **AI visibility** - Optional "show thinking" mode for debugging/fun

### Implementation Strategy

Phase approach to manage complexity:
1. Foundation (spawn + basic behavior)
2. Game logic (turns + resolution)
3. Visualization (rendering + basic interaction)
4. Polish (animations + advanced AI)

Each phase delivers value while building toward the complete system.

## Lessons Learned

1. **Don't skip the spawn engine** - It's the bridge between empty rooms and populated encounters
2. **Event-driven pays off** - Makes AI observable, testable, and debuggable
3. **Templates over code** - Data-driven behaviors are more flexible
4. **Phased delivery works** - Each project can progress semi-independently

## Next Steps

1. Implement spawn engine (ADR-0013)
2. Create behavior system ADR
3. Define encounter service protos
4. Begin phase 1 implementation

The foundation is solid. Time to build the encounter system on top.