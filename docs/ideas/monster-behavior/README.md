# Monster Behavior

> Make monsters fight back with simple, extensible behavior

## Status: Design Complete

Ready for implementation.

## Trigger

Demo goblin just stands there getting beat up. Can we build something simple and extensible?

## Goals

| Goal | Description |
|------|-------------|
| Primary (B) | Goblin picks closest enemy, moves toward, attacks, uses Nimble Escape |
| Stretch (C) | Goblin chooses Disengage vs Hide based on conditions |

## Key Decisions

| Question | Answer |
|----------|--------|
| Are monsters special? | No — same systems as characters |
| What are special abilities? | Features — same pattern |
| How select actions? | Utility scoring |
| What are monster actions? | `core.Action[MonsterActionInput]` |
| How does movement work? | Part of action execution via GameCtx.Room |
| How find targets? | Build perception from room queries |

## Architecture

```
Game Server (rpg-api)
    │
    ├─ Creates EventBus, wires entities
    │
    └─ On monster's turn:
        └─ monster.TakeTurn(ctx, TurnInput)
            ├─ Build perception from room
            ├─ Score actions, pick best
            └─ Execute via core.Action[T]
```

## Documents

| Document | Purpose |
|----------|---------|
| [brainstorm.md](brainstorm.md) | Initial ideas and exploration |
| [use-cases.md](use-cases.md) | 5 concrete scenarios |
| [design-monster-structure.md](design-monster-structure.md) | Full technical design |
| [goblin.json](goblin.json) | Reference data from dnd5eapi |
| progress.json | Structured tracking |

## Implementation Steps

1. Create `monster` package in `rulebooks/dnd5e/monster/`
2. Implement `Data` struct and `LoadFromData`
3. Create `MonsterAction` interface and `MonsterActionInput`
4. Implement `ScimitarAction` with `Score()`
5. Implement `TakeTurn` behavior loop
6. Wire up to encounter orchestrator in rpg-api
7. Test: goblin finds target, moves, attacks

## References

- [ADR-0016: Behavior System Architecture](../../adr/0016-behavior-system-architecture.md)
- [Journey 017: Encounter System Design](../../journey/017-encounter-system-design.md)
