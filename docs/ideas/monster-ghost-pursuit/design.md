# Monster ghost pursuit — design

**Status:** Implemented

**Execution:** [plan.md](plan.md)

**Implementation record:** [implementation.md](implementation.md)

**Product direction:** [rpg-project#201](https://github.com/KirkDiggler/rpg-project/issues/201)

**Design input, not toolkit authority:**
[rpg-project#305](https://github.com/KirkDiggler/rpg-project/issues/305) and
[rpg-project PR #306](https://github.com/KirkDiggler/rpg-project/pull/306)
remain design input. Toolkit authority is this repository's merged code,
published module graph, ADRs, and verification record.

**Rules authority:** The
[System Reference Document 5.1](https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf#page=76)
is the primary rulebook authority. [Roll20's 2024 Free Basic
Rules](https://roll20.net/compendium/dnd5e/Free%20Basic%20Rules%20%282024%29)
may clarify terminology or provide a reference, but do not override SRD 5.1.

## Outcome

A fight-time monster with no visible player may pursue the closest reachable
player position it remembers. It walks to that exact cell using only its own
Intel. If it reaches the cell and does not perceive that player there, the
player remains a known subject but their position becomes explicitly unknown.
The resolved location is no longer actionable, so the monster does not
repeatedly pursue it.

This is the first behavior rung for issue 201. It proves that a monster can act
intentionally on stale belief and correct that belief without receiving another
member's concealed live position. It is not a general monster-planning system.

## Existing contracts

- `play/intel` stores opaque, channel-sourced testimony. It does not inspect
  payloads, geometry, or world truth.
- `rulebooks/dnd5e/encounter` constructs lawful percepts, owns geometry and
  line of sight, authors location testimony, projects driver views, and detects
  arrival at a remembered cell.
- `rulebooks/dnd5e/behavior` owns target-selection policy and consumes plain
  view data. It never queries a live encounter.
- `rulebooks/dnd5e/session` remains a load-act-save host seam. It projects and
  persists results but owns no rule or behavior decision.
- Persistent belief, stale positions, and target policy are project behavior
  layered around SRD 5.1 mechanics.

`intel.Surveil` is unchanged. Its complete-percept fade removes a sustaining
channel while preserving the last payload, which is exactly how current sight
becomes held knowledge.

## Laws

- **G1 — Intel remains opaque.** Positional meaning is encoded and interpreted
  by encounter; Intel gains no geometry, truth access, or position API.
- **G2 — Currency and content are independent.** `Current`/`Held` says whether
  testimony is actively sustained. Encounter-owned location testimony says
  `Known(position)` or `Unknown`.
- **G3 — Views contain knowledge, never concealed truth.** A monster receives
  its own facts, current sightings, and actionable remembered positions, never
  a subject's hidden current position.
- **G4 — Live sight keeps its meaning.** `MonsterView.Seen` remains current
  sight Intel. Remembered positions are separate and never folded into `Seen`.
- **G5 — Ghosts are not attack targets.** Remembered entries have no
  attack-reach facts and their paths end on the exact remembered cell.
- **G6 — Visible knowledge interrupts remembered pursuit.** `behavior.Basic`
  selects a visible standing player whenever one exists, even if that visible
  branch ultimately passes for lack of movement or a reachable path.
- **G7 — First-rung remembered selection is deterministic.** With no visible
  player, `Basic` chooses the closest reachable remembered player. Equal
  distances temporarily break by subject ID.
- **G8 — First-rung correction occurs at arrival.** Exact remembered-cell
  arrival without a lawful percept of the subject replaces known coordinates
  with explicit unknown testimony; merely seeing the cell does not do so.
- **G9 — Correction preserves the subject.** Unknown location removes an
  actionable movement candidate; it does not delete the holding.
- **G10 — New sight replaces uncertainty.** A later current sighting overwrites
  unknown or stale positional testimony through latest accepted testimony.

## Location testimony

Encounter owns the strict tagged value:

```text
Known(position) | Unknown
```

| Intel status | Location state | Meaning | Actionable |
|---|---|---|---|
| Current | Known | The observer currently perceives the subject here. | Yes, as `Seen` |
| Held | Known | The observer remembers or was told this position. | Yes, as `Remembered` |
| Held | Unknown | The observer knows the subject but has no usable position. | No |

`Current + Unknown` is rejected. Zero and negative positions are valid; nil,
empty, malformed, or magic coordinates are not unknown sentinels. Legacy
coordinates remain readable as known; canonical writes are tagged. The
testimony is channel-neutral even though today's type is named `SightPayload`.

## Behavior and correction

For each driver call, `behavior.Basic` evaluates a fresh view and decides:

1. Select the nearest visible standing player, with the existing deterministic
   ID tie-break.
2. Attack that visible player when legal; otherwise move one cell along that
   visible path when movement remains.
3. Only when no visible standing player exists, select the nearest reachable
   remembered player; equal distances temporarily break by subject ID.
4. Move one cell along the remembered exact-cell path when movement remains.
5. Otherwise pass.

Each emitted move is one cell and encounter rebuilds the view after movement.
Consequently, refreshed lawful sight interrupts remembered pursuit on the next
driver call. The slice adds no sticky commitment.

After movement and sight refresh, encounter reads only the mover's held
location testimony, finds known positions equal to the arrival cell, and
compares their subjects with the mover's complete lawful percept. For each
absent subject it reports `Unknown` through Sight while retaining `Held`.
The correction uses neither a hidden live position nor behavior/session
interpretation, and it is surfaced through encounter-owned `IntelDelta` rather
than mutated silently. Public `Step` and free-roam `Pump` do not correct
arrival knowledge.

## Acceptance evidence

The discriminating double-door proof establishes that the monster sees Billy at
the carpet, loses sight through the second door, retains held known-at-carpet
Intel without a hidden live cell, walks to the exact carpet cell, changes the
absent Billy testimony to unknown, and then does not pursue the carpet again.
A separate test proves a newly visible player interrupts the ghost path on the
next driver call. The proof does not require finding Billy, choosing a later
door, or a fixed turn count.

Known and unknown testimony round-trip; `(0,0)` remains valid; malformed and
contradictory payloads are rejected. Current known projects only into `Seen`,
held known only into `Remembered`, and held unknown into neither actionable
collection. Session preserves load-act-save atomicity: failed persistence keeps
the prior known location and publishes no correction; success persists held
unknown.

## Deferred behavior

Multi-target inputs are supported, but multi-target sentience is deferred. The
future policy may prefer a visible-target route that also checks remembered
Intel, compare direct investigation with visible-target cost, investigate on an
equal cost, and replace the temporary ID tie-break with better authored facts.
Persisted commitment is deferred until a multi-target test demonstrates
oscillation that stateless selection cannot represent.

Also deferred: route-utility scoring, correction merely on lawful observation,
freshness or channel reliability, hiding, invisibility, sound physics,
deception behavior, search after unknown position, and general planning.
