# Issue 201 update — first monster behavior implemented

The first implementation slice for this journey is complete in the composable
encounter/session stack. The mainline integration releases and verification are
recorded in [implementation.md](implementation.md); this document retains the
approved behavior contract and the remaining project-level observation.

## The behavior

A fight-time monster that loses sight of a player may act on the position it
remembers. When no player is currently visible, it walks to the closest
actionable remembered player position. If it reaches that exact cell and does
not perceive the player there, the player remains a known subject but their
position becomes explicitly unknown. On the next decision the resolved
location is not actionable, so the monster does not pursue the same empty cell
forever.

This is one behavior rung, not a general monster-AI system. It proves both
missing halves: a decision on stale belief and belief correction on exact-cell
arrival, without exposing concealed live truth.

## Ownership and authority

- `play/intel` remains generic, opaque testimony storage; it owns no position,
  geometry, or truth interpretation.
- Encounter owns location testimony, lawful perception, exact-cell paths,
  arrival detection, and corrective testimony.
- `behavior.Basic` owns visible-first / remembered-fallback selection from
  plain view data.
- Session mirrors and persists through load-act-save; it owns no behavior rule.
- [SRD 5.1](https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf#page=76)
  is the primary rulebook authority. [Roll20 2024](https://roll20.net/compendium/dnd5e/Free%20Basic%20Rules%20%282024%29)
  is clarification or reference only.
- [rpg-project#305](https://github.com/KirkDiggler/rpg-project/issues/305) and
  [rpg-project PR #306](https://github.com/KirkDiggler/rpg-project/pull/306)
  remain design input, not toolkit authority.

Encounter may use world state to construct the monster's lawful percept, but it
never exposes a concealed player's live position. Correction derives from the
monster's prior Intel, its arrival cell, and its complete percept.

## Location knowledge and selection policy

Encounter-owned location testimony is `Known(position) | Unknown`, independent
from Intel currency. Current-known appears through `Seen`; held-known through
`Remembered`; held-unknown remains persisted but is not actionable. Tagged
canonical payloads remain backward compatible with legacy coordinates; nil,
empty, malformed, or magic coordinates are not unknown sentinels.

`MonsterView.Remembered` contains only subject/member ID, kind, remembered
position, distance, and a shortest exact-cell path. It carries neither hidden
standing state nor attack reach; ghosts cannot be attacked.

The approved policy is:

1. Visible standing targets take priority.
2. Choose the nearest visible target and attack or move according to the
   visible branch.
3. Only if none is visible, choose the nearest reachable remembered target.
4. Pursue its exact remembered cell, breaking equal remembered distances by a
   temporary deterministic subject-ID tie-break.
5. A lawful refreshed sighting interrupts remembered pursuit on the next call.

The tie-break is intentionally temporary. Richer multi-target sentience,
route-information scoring, and persisted commitment remain deferred.

## Delivered proof and remaining observation

The session double-door proof establishes held known-at-carpet memory, no
concealed live cell in the driver view, exact-cell pursuit, held-unknown
correction on lawful arrival, and no repeat pursuit. The visible-interruption
proof establishes priority for newly refreshed sight.

Before [rpg-project#201](https://github.com/KirkDiggler/rpg-project/issues/201)
can close, one smallest local-game observation remains: in the local dev
dungeon, the monster sees the hero, the hero breaks sight through the second
door, the monster walks to the last-seen carpet cell rather than hidden live
truth, and after checking that cell it does not repeat the stale move. Finding
the hero afterward is outside this slice.
