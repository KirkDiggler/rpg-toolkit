# mind/behavior — Scenarios (the arrows)

**Status:** LIVING
**Why:** the finished product is a composable, extensible monster behaviour
system. Each scenario here is a scene a player will see, and every slice
is judged by whether it brings one closer. A scenario names *what the
monster does*; it never says *where that is configured*. Keeping those
apart is the point of this file: the pieces a scene pulls on have
different owners, and a knob that lands on the wrong owner is a weight
slider through the back door.

## Arrow 1 — the bow skeleton turns (SHIPPED)

A bow skeleton shooting at the closest player turns on the one who shot
at it, and goes back to closest when that player puts the bow away and
closes to melee. [design.md](design.md) is its record; the Retaliator is
its mind; [adoption.md](adoption.md) is how the encounter pays for it.

## Arrow 2 — the goblin ducks out

A goblin fighting in a doorway, at some stage, disengages, slips into the
next room, and hides where nobody can see it. The party has to go find
it. Goblins carry Nimble Escape (SRD: Disengage or Hide as a bonus
action); the trait is the rules hook, not the design.

The scene pulls on four pieces, and none of them is the Retaliator's.

| Piece | What the goblin needs | Owner | Today |
|---|---|---|---|
| A trigger | "at some stage": bloodied, outnumbered, or just having struck. A judgement over the goblin's own state and what it holds. | The mind's `Rank` inputs — profile material. | Excuse and patience are profile fields (#1745); provocation is still hardcoded as "an attack on me", and the goblin's own trigger does not exist. |
| A new outcome | "become unseen" is not Away, Attack, Toward, or Pass. | The ladder, as a claim beside `Keep` — not the mind's to reorder (R‑ladder, [design.md](design.md)). | No such rung. This is the first use case that earns one. |
| A place to go | a cell no Named contact currently sees, reachable this turn. | `mind/perception`: the mind asks, perception answers. Behaviour never reasons about sight itself (R1). | Concealment exists in `world/graph` and the encounter; no "cells unseen by these observers" query. |
| A two-step turn | disengage, move there, hide: a bonus action beside a move, compelled as one turn. | The driver (`rulebooks/dnd5e/encounter`, session). | The driver issues one intent per turn; `disengage` and `dash` exist as actions, `hide` does not. |

### What is configured where

- **Profile** (the mind): what the monster cares about. Excuse (the actor
  unseen or holding ranged; any weapon), patience (how long a grudge
  lasts), and how much room it wants are fields as of #1745, and the zero
  value tells the truth: `Grudge{}` is a mind that holds no grudge and says
  so. **Provocation is UNPAID** — an attack on me is hardcoded, and an
  attack on an ally or a damage-only rule has no mind asking for it. So is
  the goblin's own trigger. Neither self-extends from the profile slice;
  each waits for the scene that wants it.
- **Claims** (the ladder): what wins. `Keep` today; `Sole` (narrow rung 1
  to one target so a grudge is chased past the fighter in front) and
  the goblin's "become unseen" are the candidates. A leash ("stop
  chasing after thirty feet") is a claim on Toward, not a profile field.
- **Perception**: what can be seen, and from where.
- **Driver**: how a turn is spent.

"How far the monster takes it" is therefore never a profile field. It
is the ladder abandoning one rung for another, and only a claim may say
that.

### Order of work

1. **SHIPPED** ([#1745](https://github.com/KirkDiggler/rpg-toolkit/issues/1745);
   PRs [#1746](https://github.com/KirkDiggler/rpg-toolkit/pull/1746),
   [#1748](https://github.com/KirkDiggler/rpg-toolkit/pull/1748),
   [#1750](https://github.com/KirkDiggler/rpg-toolkit/pull/1750)) — a
   tunable Retaliator profile: `Grudge{Patience, Excuse}` and `Room`,
   proven by three minds on one scene in the driver's own tests. Berserker
   (any weapon, long patience), bow skeleton (ranged only, short patience),
   coward (no grudge, two steps of room). Provocation was NOT part of it and
   is recorded as unpaid above. The fields a profile has are now known,
   which is what had to be true before a format is chosen; authoring minds
   as data stays a non-goal, and its price is now measurable — every tune is
   a new word and a toolkit release.
2. `Sole`, if the walk shows the chase is wanted.
3. The goblin's four pieces, each with its own use case and owner, in
   whatever order the walk demands. None of them self-extends from the
   profile slice.

### Open

- The walk that proves a profile is two monsters with different words in
  one room. The driver files a mind per MEMBER id, not per session, so the
  mechanism is there and #1745's session PR pins it; the walk itself is
  still owed.
- `docs/ideas/monster-behavior/` is the pre-perception design (utility
  scoring, `monster.TakeTurn`) for this same goblin. It is superseded by
  this line and kept as history.
