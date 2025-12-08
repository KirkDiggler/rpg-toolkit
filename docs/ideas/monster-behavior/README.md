# Monster Behavior

> Make monsters fight back with simple, extensible behavior

## Status: Brainstorming

## Trigger

We have a goblin in a demo room that does nothing but get beat up. Can we build something simple and extensibly?

## Goals

| Goal | Description |
|------|-------------|
| Primary (B) | Goblin picks closest enemy, moves toward, attacks, uses Nimble Escape |
| Stretch (C) | Goblin chooses Disengage vs Hide based on conditions |

## Key Decisions

| Question | Answer |
|----------|--------|
| Are monsters special? | No — same systems as characters (action economy, features, conditions) |
| What are special abilities? | Features — Nimble Escape works like Rage or Second Wind |
| How select actions? | Utility scoring — each action scored, highest valid wins |
| What are actions? | Rich objects with cost, range, triggers, effects |

## Documents

| Document | Purpose |
|----------|---------|
| [brainstorm.md](brainstorm.md) | All ideas - practical, dreamy, crazy |
| [use-cases.md](use-cases.md) | Concrete scenarios end-to-end |
| [goblin.json](goblin.json) | Reference data from dnd5eapi |
| progress.json | Structured tracking |

## Architecture

```
rpg-api (orchestrator) → dnd5e rulebook (game logic) → toolkit (infrastructure)
                              ↑
                        Goblin behavior lives here
```

## Use Cases

1. **Basic Melee Attack** — Move toward player, attack with scimitar
2. **Ranged Preference** — Stay at distance, use shortbow, hide behind cover
3. **Tactical Retreat** — Low HP + surrounded → Disengage and flee
4. **Ambush Setup** — Hide behind cover for advantage on next attack
5. **Healing Potion** — Critically wounded → prioritize survival

## References

- [ADR-0016: Behavior System Architecture](../../adr/0016-behavior-system-architecture.md)
- [Journey 017: Encounter System Design](../../journey/017-encounter-system-design.md)
