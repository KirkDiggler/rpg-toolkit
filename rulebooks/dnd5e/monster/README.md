# D&D 5e monsters: current guide

This directory is the nearest orientation point for monster **content** — stat
blocks, actions, traits, refs. It is inside the single `rulebooks/dnd5e` Go
module.

> ### Working on monster *behavior*? Start somewhere else.
>
> Behavior has a new seam, and it is not `TakeTurn`. Read
> **[`rulebooks/dnd5e/encounter/README.md`](../encounter/README.md)** →
> *Writing a decider*.
>
> The short version: behavior is a `Decider` — one method, `Decide(Snapshot)
> (Intent, error)`. A `Snapshot` gives you **your own room, your own position,
> your own held intel and nothing else**, so a decider structurally cannot read
> the world or another member's truth (the anti-wall-hack contract, C2). You
> return an *intent* — move, traverse, or hold — and the composition decides
> whether it happens. That makes a decider a pure function you can unit-test
> with a plain struct and no encounter at all.
>
> **Honest limit:** there is no attack intent yet. Movement, pursuit, patrol and
> traversal are expressible today; attacking and targeting are wave-4 work.
>
> Everything below about **`NPCAct`, `TakeTurn`, and the wired `CombatResolver`
> describes the OLD encounter module** — the stack that runs the shipped game.
> It is accurate about that stack and useful if you are working in it. It is not
> what new work builds on, and the two do not share a world model.

The **content** model below is current either way: a new monster still needs a
factory, a canonical ref, actions and traits, whichever stack drives it.

## Current packages

| Import path suffix | Current responsibility |
|---|---|
| `monster` | `Monster` runtime type, `Data`, action/perception contracts, targeting, `TakeTurn`, pure `Load` + `SheetKeeper`, legacy `LoadFromData` / `ToData` |
| `monster/actions` | Loadable generic melee, ranged, multiattack, and bite action implementations plus the action loader |
| `monster/monsters` | Built-in stat factories (including `NewGoblin`) and the canonical-ref-to-constructor registry |
| `monstertraits` | Loadable condition-style monster traits (separate sibling package to avoid import cycles) |
| `refs` | Canonical monster, monster-action, and monster-trait refs |

In Go import terms, `monster/actions` and `monster/monsters` are subpackages,
not symbols exposed through package `monster`; callers import them directly.

## Four responsibilities that must not be collapsed

```text
definition         tactics / decision          rules resolution        encounter composition
factory + ref  →  Monster.TakeTurn        →  combat resolver      →  encounter.NPCAct
stat/action data    scores + target strategy    hit/damage verdicts     state, movement, events
```

### 1. Definition

A built-in factory in `monster/monsters` composes a runtime `*monster.Monster`
from current capabilities. It gives the creature a canonical ref, stats,
actions, speed, optional supported trait JSON, and optional targeting. The
registry maps the full ref string to that constructor so authored encounter
content can resolve it.

A definition should contain no storage or encounter orchestration.

### 2. Tactics / decision

The shipped decider is `(*monster.Monster).TakeTurn`; there is no separate,
usable behavior-tree or tactics module today. `TakeTurn` currently:

- chooses a target using `TargetClosest`, `TargetLowestHP`, or `TargetLowestAC`
  (`TargetingUnspecified` behaves like closest at decision time);
- uses `tools/spatial.NewSimplePathFinder` to move toward that same target,
  respecting blocked cells and an optional traversal predicate;
- scores affordable `MonsterAction`s, chooses the highest score, selects a
  target appropriate to the action type, activates it, and consumes the
  matching action-economy slot;
- returns the movement path and attempted-action records.

Action scoring and target selection are intentionally simple. The top-level
`behavior/` directory contains only a design stub, not a Go module or shipped
behavior framework.

### 3. Rules resolution

The generic melee/ranged/bite actions validate range and publish a D&D 5e
`AttackEvent`. They do not themselves roll the complete attack and apply damage.
The current top-level encounter SDK captures that event and delegates hit/damage
to its wired `CombatResolver`.

Do not infer behavior from a config field alone:

- melee/ranged attack bonus, damage dice, and damage type serialize and are used
  by current encounter seeding snapshots, but the action's `Activate` method is
  event publication, not standalone damage resolution;
- `BiteConfig.KnockdownDC` round-trips, but bite knockdown is explicitly not
  implemented;
- ranged activation enforces long range, but normal-range/long-range
  disadvantage behavior is not implemented there;
- `PackTactics` loads and subscribes but its ally-adjacency modifier is a no-op;
- `UndeadFortitude` rolls a save but does not restore/mutate HP, and it cannot
  detect critical hits from the current event;
- a factory comment saying a monster “has” one of those traits does not attach
  or complete the behavior. Verify `AddTraitData` and the trait implementation.

Immunity and vulnerability traits do have chain modifier implementations and
are attached by factories that call their `Must...JSON` helpers. Every new
clause still needs a focused test on the real event path.

### 4. Encounter composition

The current top-level `encounter` module is D&D-5e-coupled. `NPCAct` builds
perception and action economy, calls the shipped `TakeTurn`, applies movement,
captures rulebook attack/condition events, resolves attacks through the
encounter's resolver, updates encounter state, and publishes encounter events.
It also requires a rehydratable monster JSON blob; the old scripted fallback is
not a supported path.

Monster definitions should not reach into encounter composition. Encounter
selection, group size, placement, difficulty budgeting, room theme, and spawn
frequency are separate authoring/composition concerns.

## Construction and loading are not the same operation

A factory returns a ready-to-serialize runtime monster:

```text
monster/monsters constructor → *monster.Monster → ToData / JSON
```

Reload comes in two shapes. The one to write new code against is a pure load
and an attach, and neither can be two-thirds made:

```text
monstertraits.LoadMonster(ctx, data)             # sheet + actions + the trait blobs, no bus
monstertraits.AttachMonster(ctx, mon, bus, roller)  # sheet keeper, then each trait, attributed
```

`LoadMonster(data).ToData()` is the data it was given, actions and conditions
included, with no bus anywhere in the call. The composition lives in
`monstertraits` for a mechanical reason: package `monster` cannot import either
loader (both import it), and `monstertraits` is the only package that can see
both without a cycle.

The older shape is the three-call assembly, still used by existing callers:

```text
monster.LoadFromData(ctx, data, bus)             # base state + monster subscriptions
actions.LoadMonsterActions(mon, data.Actions)    # runtime action implementations
monstertraits.LoadMonsterConditions(             # trait JSON → Apply on bus → attach
    ctx, mon, data.Conditions, bus, roller,
)
```

The encounter's `LoadFromData` hydration cascade performs all three and later
writes held combatants back through `ToData`. Outside the encounter, the caller
must perform the extra action and trait steps. A test that calls only
`monster.LoadFromData` has not proven a full monster round trip — and neither
has production code: `ToData` serializes actions and conditions, so a monster
assembled by two of the three calls is written back with the third's contents
silently gone. That trap is the reason the pair above exists.

## Supported first contribution

Use [`monster/monsters/bandit.go`](monsters/bandit.go) and
[`bandit_test.go`](monsters/bandit_test.go) as the canonical simple **structural**
fixture: one factory, one supported generic attack, no special trait clause,
explicit speed, and direct stat/action assertions. Do not copy its older
distance literals: values such as `Reach: 5` and `RangeNormal: 80` are
feet-shaped, while current action configs and `PerceivedEntity.Distance` use
hex counts (5 feet = 1 hex). New content must use and test current units.
Also follow [`registry.go`](monsters/registry.go) and
[`registry_test.go`](monsters/registry_test.go), which prove that each registered
ref constructs a monster carrying the same ref.

The full operator contract, source rules, exact file checklist, and round-trip
acceptance are in [Add a D&D 5e monster](../../../docs/how-to/add-a-dnd5e-monster.md).

## Historical and proposed documents

These are context, not current instructions:

- [Monster structure design](../../../docs/ideas/monster-behavior/design-monster-structure.md)
  is a historical design note; several snippets differ from shipped APIs.
- [Monster additions design](../../../docs/plans/2025-12-15-monster-additions-design.md)
  is an implementation-era plan; some roster entries shipped while some stated
  clauses remain unsupported.
- [Monster pathfinding plan](../../../docs/plans/2026-01-06-monster-pathfinding.md)
  is historical; shipped A* now lives in `tools/spatial` and `TakeTurn` uses it.

### Proposed follow-up: flatten built-in content by one package

A desired future information architecture is
`rulebooks/dnd5e/monsters` as an immediate package inside the existing
`rulebooks/dnd5e` module. It would move the built-in factories/registry out of
`monster/monsters` while leaving runtime types in `monster`.

This is **not the current path**, has not been implemented here, and must not get
its own `go.mod`. It requires a separately reviewed migration with import and
consumer updates. Until that lands, add content to the current paths documented
above.
