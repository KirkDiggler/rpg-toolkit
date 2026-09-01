# ADR-0046: Generic NPCs Are Content

Date: 2026-09-01

## Status

Proposed

## Context

The toolkit needs world NPC support for vendors and future non-player entities.
The first concrete consumer is a D&D 5e vendor, but the idea of an NPC is not
D&D-specific. A merchant, guide, hostage, guard, hireling, informant, or neutral
person-like world object may appear in many rulebooks and hosts.

The current D&D encounter package lives under `rulebooks/dnd5e/encounter`, even
though much of its field and member machinery is generic in spirit. That boundary
is already recognized as awkward, but moving encounter is too large for this
work. NPCs should not repeat the mistake by putting generic identity and policy
inside the D&D rulebook just because the first NPC is a D&D vendor.

NPC placement also needs a movement answer. Today, `tools/spatial` asks cell
occupants through `Placeable.BlocksMovement() bool`, and encounter props/walls
ultimately compile down to binary movement blocking. That answers the immediate
placement question, but it cannot express future cases where a cell occupant may
block enemies while allowing allies, or otherwise depends on the mover.
Spatial already has a richer precedent in another lane: room connections receive
the moving entity through `IsPassable(entity core.Entity) bool`.

## Decision

Create a top-level `npc` module/package for generic NPC content. It owns the
shared content shape: stable `*core.Ref`, display name, opaque capabilities,
combat policy, observation policy, disposition policy, and movement policy.

Rulebook-specific vendors live in the rulebook. A D&D merchant may compose with
`npc.NPC`, but D&D owns D&D item refs, stock rules, prices, purchase flow, and
inventory mutation.

World and encounter integration live outside `npc`. The `world` package owns
living-world graph facts, verbs, goals, and journal projection. The D&D encounter
package owns placed runtime facts such as member ID, position, current visibility
knowledge, combat membership, and current movement blocking.

NPC movement is authored as a named policy, not a bool:

```go
type MovementPolicy string

const (
    MovementPolicyBlocking MovementPolicy = "blocking"
    MovementPolicyPassable MovementPolicy = "passable"
)
```

Today's encounter/spatial adapter may map that policy to `BlocksMovement() bool`.
That is an adapter decision at the current spatial seam, not the generic NPC
model. Future spatial or encounter work may add mover-aware cell occupancy,
similar to connection passability, without renaming NPC content.

Rejected options:

- Put NPCs under `rulebooks/dnd5e`: rejected because NPC identity is generic and
  would make non-D&D consumers import a rulebook.
- Name the reusable content type `Definition`: rejected because the thing being
  modeled is the NPC content record itself, and nearby content packages already
  use concrete domain nouns for primary records.
- Store `BlocksMovement bool` on `npc.NPC`: rejected because it bakes today's
  spatial cell-occupancy limit into generic content and hides future
  mover-dependent policies behind a boolean name.
- Add `MovementPolicyAlliedPassable` now: rejected because current spatial cell
  occupancy cannot evaluate mover-vs-occupant, and no concrete consumer has
  specified the team/hostility semantics yet.

## Consequences

### Positive

- Generic NPC identity can be shared by multiple rulebooks and hosts.
- D&D vendors can build on generic NPC content without moving shop behavior into
  the generic package.
- NPC content stays compatible with the current binary spatial seam while
  preserving a path to richer movement semantics.
- Unknown capabilities can round-trip without forcing the generic package to
  name every future NPC role.

### Negative

- The first encounter integration still needs an adapter from `MovementPolicy`
  to `BlocksMovement() bool`.
- Team-aware or mover-aware movement remains unimplemented until spatial or
  encounter grows that seam.
- Another top-level module adds versioning overhead.

### Neutral

- `npc` does not decide hostility, teams, factions, sight, sound, smell,
  inventory, pricing, dialogue, or living-world projection.
- The first vendor should use `MovementPolicyBlocking`,
  `CombatPolicyNonCombatant`, a vendor capability, and neutral disposition.
- Existing D&D encounter member-kind decisions remain outside this ADR.
