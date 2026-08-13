# D&D 5e Rulebook Development Guidelines

This document contains patterns specific to the dnd5e rulebook module.

## Live play — who plays which part

The three-kind layering is described in the root `CLAUDE.md` ("How live play is
layered"). In this rulebook the parts are currently cast like this:

- **`encounter/` is the composition** — a composable approach to encounter
  tracking. It is the courier between the `play/*` leaves (`clock`, `intel`,
  `interrupt`, `record`) and `tools/spatial`: it surveils percepts into intel,
  lets deciders act on their own intel, appends the story to record, and pumps
  the clock. **Game rules and trigger detection belong here** — it is the first
  layer allowed an opinion about D&D. Laws C1–C8, plus anchoring W1–W5.

- **`session/` is the host seam** — rpg-api's one interface to the toolkit.
  Verbs take IDs, repositories are key-value (S12), every verb loads-acts-saves
  with no session process (S4), and no runtime object crosses the boundary (S2,
  enforced by `TestNoInnerTypeCrossesTheBoundary`). **It owns no rules.** A rule
  found here is misplaced, not convenient.

- **`combat/`, `character/`, `conditions/`, `features/` … are the rules.** They
  know what Rage does. They do not know what a session is.

**Time, turns, and "whose turn is it" are not this rulebook's invention.** They
live in `play/clock`: `Tick` is the player-driven world clock, `Turn` is a
localized initiative bubble, and `Transfer` moves an entity between them
atomically under R6 (an entity belongs to at most one clock). There is no
`Mode` enum anywhere in the new stack, and `FREE_ROAM`/`TURN_BASED` is an
artifact of the *old* encounter module rather than a D&D concept. Read
`play/README.md` and `play/clock`'s godoc before designing anything that
touches turns, rounds, or initiative — the vocabulary already exists.

_(`initiative/` in this module is dead — imported by nothing. `play/clock` superseded it.)_

## Refs Pattern

**Everything has a Ref.** All identifiable content (features, conditions, combat abilities, actions, weapons, etc.) must have a `*core.Ref` with `{Module, Type, ID}`.

- Module: `"dnd5e"` (enables future modules like `"artificer"` without ID conflicts)
- Type: Category (`"features"`, `"conditions"`, `"actions"`, etc.)
- ID: Specific identifier (`"rage"`, `"dodging"`, `"strike"`)

**Game server uses Refs** to identify what to activate/check. Proto enum maps to Ref.

**Loaders reconstitute behavior.** Refs identify the module/package, then:
- `LoadFromData(data)` - When types share a consistent schema (e.g., actions)
- `LoadJSON(data)` - When types have unique state structures (e.g., features, conditions)

Choose based on whether the data structure is homogeneous or heterogeneous.

**Refs namespace pattern** (`refs/` package):
```go
refs.Features.Rage()           // *core.Ref for Rage feature
refs.Conditions.Dodging()      // *core.Ref for Dodging condition
refs.CombatAbilities.Attack()  // *core.Ref for Attack ability
refs.Actions.Strike()          // *core.Ref for Strike action
```

## Conditions vs Effects

Conditions are really "effects" but we're saving that rename for 1.0. Use `character.HasCondition(refs.Conditions.Dodging())` to check for active effects like Dodging or Disengaging.

## Two-Level Action Economy

D&D 5e has two levels of resource consumption:

1. **Action Economy** - What you spend (action, bonus action, reaction)
2. **Capacity** - What you get to do (attacks, movement, off-hand attacks, flurry strikes)

Example: Taking the Attack ability (spends action) grants attacks (capacity). Each Strike action consumes one attack from that capacity.

Key fields in `ActionEconomy`:
- Primary: `ActionsRemaining`, `BonusActionsRemaining`, `ReactionsRemaining`
- Capacity: `AttacksRemaining`, `MovementRemaining`, `OffHandAttacksRemaining`, `FlurryStrikesRemaining`
