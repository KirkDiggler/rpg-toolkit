# World NPCs - Design

**Status:** APPROVED (2026-09-02) — encounter/session integration slice
tracked as [rpg-toolkit#1404](https://github.com/KirkDiggler/rpg-toolkit/issues/1404)
**Issue:** [rpg-toolkit#1280](https://github.com/KirkDiggler/rpg-toolkit/issues/1280)
**Modules:** `npc`, `rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/session`
**Why:** `brainstorm.md`. **How:** `plan.md`.

**Implementation note:** the first shipped slice is the generic `npc` package
described in [../npc](../npc/). Encounter/session integration remains proposed
future work in this document.

## Amendment (2026-09-01) — disposition is a graph relation, not a policy enum

Recorded before Task 3 of `plan.md` starts, per Task 1's gate ("record any
design amendment needed before implementation").

**What changed:** `examples/world/scenarios/banditcamp/camp.go` already proves
a working disposition mechanism — `HostileTo` and `AlliedWith` declared as
plain `graph.Relation` edges between `KindFaction` entities, moved over time by
ordinary facts (a parley's `Regard` counter crossing a threshold, `Retire`
stripping an edge from a defeated faction). `rulebooks/dnd5e/encounter/conceal.go`
independently proves the exact hosting shape this needs: `encounterWorld`
already embeds a scoped `graph.World` plus `journal.Journal`, seeded once from
compiled/authored data, with present state folded fresh on every question and
nothing cached. Both pieces predate this idea and were not designed for it, but
they compose into a disposition model without inventing anything new.

**What this supersedes:** the `NPCDispositionPolicy` sketch below (Encounter
Model) is downgraded from "the runtime hostility mechanism" to an authoring
default only — the word an author or a loaded profile starts with. The live
question `classify()` asks — is this member's side hostile to that one — is
answered by folding `HostileTo`/`AlliedWith` edges the same way `knowsDoor`
folds concealment, never by switching on `MemberKind` or reading a stored
enum directly. "Neutral" is not a stored state under this model; it is the
absence of an edge, which is also why later shades (frightened, charmed,
escorted, temporarily allied — already anticipated in this doc's disposition
paragraph below) cost a new `Relation`/`Flag` declaration at the point some
rulebook needs one, never a new toolkit type or a widened enum.

**Correction (2026-09-01), after review:** none of this is required to ship
this idea's MVP, and it does not gate Task 3. `KindWorld`'s neutrality falls
out of `sidesInContactOrder`'s existing switch for free — see the Combat
Exclusion section below. The graph-relation mechanism only has a consumer
once something needs a *non-neutral* `KindWorld` NPC, or a `KindMonster`
that isn't unconditionally hostile, both of which this doc lists as
non-goals. Recording it here is forward context for that later idea, not a
decision this one has to make. If that later idea does pick it up, the
question it will face is where the disposition graph lives:

- **Encounter-scoped**, seeded at Setup exactly the way `conceal.go` seeds
  concealment from the compiled field — hostility authored per encounter,
  not surviving past it; or
- **World-owned**, computed by a governing `world` layer above both
  `examples/world`'s living-world track and `rulebooks/dnd5e/encounter`'s
  combat track (disconnected today) and handed down at Setup/Pump — the
  shape that would let a bandit camp's disposition, set during exploration,
  carry into the fight it triggers.

Left unresolved here on purpose, for whichever idea actually needs it.

## Amendment (2026-09-02) — encounter carries no NPC content at all

This is a bigger correction than the ones above: most of this document,
written before implementation started, modeled a world NPC as a "living
world" entity — content (ref, capabilities, combat/observation/disposition
policy) riding on the *encounter* member itself, with its own location-known
semantics (N8) and its own observation policy (N9). That is not what a
monster is, and it is not what this slice needs.

**What a monster actually does today, traced directly (`session/write.go`):**
`session.Spawn` resolves `in.Ref` into a `monster.Data` sheet, stores that
sheet in `scope.data.NPCs` — a **session-owned** store keyed by member ID —
and calls `encounter.Join` with only bare resolved facts: `Name`, `Position`,
`Speed.Walk`, `Darkvision`, `Actions`, `Targeting`. **No ref, no content, no
policy of any kind crosses into `encounter`.** `encounter` does not know what
a monster *is* — only that something with these facts stands here. Combat
resolution later reads the sheet back out of `SessionData.NPCs` by member ID;
`encounter` is never asked again.

A world NPC should be placed exactly the same way, because "not being a
combatant" is a `MemberKind` fact (`KindWorld` is on neither side of
`sidesInContactOrder`, structurally, for free — see Combat Exclusion), not a
content fact. So:

- **`encounter` gains `KindWorld` and nothing else new on `MemberInput`.** A
  `KindWorld` member carries the same bare fields a player or monster does
  (`ID`, `Name`, `Position`, `SpeedFeet`, `SightFeet`, `Actions`,
  `Targeting`) — typically zero/empty for a stationary, non-acting NPC, by
  caller choice, not by new validation `encounter` enforces. The only new
  check mirrors the one existing kind-specific rule this package already has
  (`KindPlayer` + `Decider != nil` → reject): `KindWorld` + `Decider != nil`
  → reject, same shape, nothing more. `encounter` does **not** import `npc`.
  There is no `Ref`, `Capabilities`, `CombatPolicy`, `ObservationPolicy`,
  `MovementPolicy`, or `DispositionPolicy` field anywhere in `encounter`'s
  `MemberInput`/`JoinInput`/`Member`/`memberRecord`/`MemberData`.
- **`session` owns the actual NPC content**, in a new store parallel to
  `SessionData.NPCs` (which is monster sheets, by an unfortunate existing
  name — see N1/Session Seam for why this needs a different field name),
  keyed by member ID: the placed `npc.NPC`/`npc.Data` this member was spawned
  from. This is exactly where a monster's sheet already lives, and it is
  where `Ref`, `Capabilities`, and the three policy words belong too. Only
  `session` ever imports `npc`'s content types for this purpose.
- **Movement blocking is cut from this slice.** Checked directly:
  `memberEntity.BlocksMovement()` returns `false` unconditionally for *every*
  member today — a player and a monster already don't block movement through
  this hook (occupancy exclusivity, "nobody else can stand exactly here", is
  a separate tools/spatial concern, not this one). "Place it the same way as
  a monster" means a `KindWorld` NPC gets the same answer a monster already
  gets: `false`. A blocking shopkeeper is a real future increment, not
  something to build now by making NPCs different from monsters on day one.
  This retires N5's `MovementPolicy` claim and the whole "Placement and
  Blocking" section's blocking behavior for this slice.
- **N8 (locations are learned, not assumed) and N9 (observation policy) are
  deferred, not part of this slice.** N8 describes exactly what already
  happens for *any* member through ordinary sight/intel — it is not a new
  NPC-specific mechanism, and does not need a policy field to be true. N9's
  observer/subject-only distinction has no consumer yet (nothing here builds
  NPC-driven behavior); if a future NPC ever needs to hold its own intel,
  that is exactly what giving it a nonzero `SightFeet` already does for a
  monster, with no new policy concept required.
- **The interaction descriptor is built by `session` from its own store, not
  from anything `encounter` reports.** `encounter`'s role for interaction is
  the same kind of thing it already does for a monster: confirm adjacency,
  visibility, and identity by member ID. What the NPC *is* — name,
  capabilities, policy — is a `session`-side lookup by that same ID, exactly
  parallel to how `session.Attack` reads a monster's sheet out of
  `SessionData.NPCs` rather than asking `encounter` what kind of monster it
  placed.

Everything else in this document not touched by the above still holds: N1–N4,
N6–N7, N10–N11, the package split, the non-goals, and the disposition
amendment's own correction (not needed for this slice either). The sections
below are edited in place to match — this correction is not layered on top of
them as a second thing to reconcile at implementation time.

## Scope

Add first-class world NPC support to the modern D&D 5e live-play stack. This
issue builds the general placed-NPC framework that
[rpg-toolkit#1275](https://github.com/KirkDiggler/rpg-toolkit/issues/1275) can
use for its first vendor/NPC inventory implementation.

A world NPC is a placed, interactable, non-combat encounter entity. It stands on
the same dungeon-absolute canvas as players and monsters, placed by `encounter`
with the same bare facts a monster is (see the 2026-09-02 amendment above — no
content crosses into `encounter`), and can be interacted with by a nearby
player. It is not a monster, does not enter fight bubbles, takes no turns, and
is not an attack target in the MVP. Movement blocking is out of scope for this
slice (a monster does not block movement today either — see the amendment).

Vendor inventory, buying and selling, quote calculation, stock availability,
dialogue trees, quests, trainer services, factions, NPC hit points, drops,
attackable neutral NPCs, and hostile NPCs are out of scope. Vendor inventory and
buy-only shop behavior belong to #1275.

## Laws

- **N1 - world NPCs are not monsters.** A world NPC is not a combat sheet, not
  a monster factory instance, and not stored in `SessionData.NPCs`.
- **N2 - one placement model.** World NPCs stand on the encounter's existing
  dungeon-absolute canvas. No room-local API and no second spatial registry.
- **N3 - interaction is descriptive.** Interacting returns who/what is there and
  which capabilities it exposes. It does not execute vendor, dialogue, quest, or
  trainer behavior.
- **N4 - non-combat is explicit.** The MVP has exactly one combat policy:
  non-combatant. Non-combatant NPCs are excluded from fight formation, turn
  clocks, monster targeting, player attack candidates, and attack execution.
- **N5 - movement blocking is deferred, not built this slice.** `npc.MovementPolicy`
  exists on the generic content record (already shipped), but no encounter/spatial
  adapter consumes it here — `memberEntity.BlocksMovement()` returns `false`
  for every member kind today, monsters included, and a `KindWorld` NPC gets
  the same answer (2026-09-02 amendment). A future slice may wire this
  through deliberately; this one does not claim it works.
- **N6 - capabilities are opaque.** The encounter/session layers carry
  capability words and never implement behavior behind them in this issue.
- **N7 - host seam keeps its own twins.** `session` exposes session-owned NPC
  interaction/read types rather than leaking encounter internals.
- **N8 - deferred (2026-09-02).** Originally: NPC locations are learned via
  intel, not globally known. Not a new mechanism — a `KindWorld` member is
  perceived through the same sight/intel path any member is, automatically,
  with no policy needed to make that true. Stated here only so a future
  reader does not go looking for a special case that was never built.
- **N9 - deferred (2026-09-02).** Originally: a per-NPC observation policy
  deciding whether it can hold its own intel. No consumer exists yet; a
  `KindWorld` member with a nonzero `SightFeet` already gets its own percept
  through the same mechanism a monster does, with no new policy concept.
- **N10 - vendors fit under NPCs, not beside them.** The first vendor/NPC
  inventory implementation is #1275. This issue supplies the general world-NPC
  identity, placement, visibility, interaction descriptor, and non-combat policy
  it needs. The vendor profile proves the framework without making every future
  world NPC always known or vendor-shaped.
- **N11 - definitions and placement are different responsibilities.** Reusable
  NPC facts live in a toolkit-level `npc` package. Placed D&D encounter
  behavior lives in `rulebooks/dnd5e/encounter`. Host-shaped D&D orchestration
  lives in `rulebooks/dnd5e/session`.

## Package Ownership

World NPC support should not be only an `encounter` feature, should not live
inside a rulebook, and should not reuse the `monster` package.

Use this split:

| Package | Responsibility |
|---|---|
| `npc` | Toolkit-level generic NPC data: `NPC`, `Data`, load/validation, `*core.Ref`, display name, interaction capabilities, combat policy, observation policy, disposition policy, and movement policy. No shop stock, no combat sheet, no turn behavior, and no D&D-only rules. |
| `npc/npcs` | Deferred until implementation or #1275 proves a built-in/common profile registry is needed. |
| `rulebooks/dnd5e/vendors` or nearest existing D&D content package | D&D vendor types/profiles that compose with `npc`: D&D item refs, stock defaults, pricing assumptions, vendor category data, and #1275 inventory behavior hooks. |
| `rulebooks/dnd5e/encounter` | Runtime placement and D&D live-play behavior: `KindWorld` member kind, cell, interaction range/visibility checks, story beat, and combat exclusion. Carries no NPC content — bare member facts only, exactly as for a monster (2026-09-02 amendment). |
| `rulebooks/dnd5e/session` | Host seam: start/load/place/interact inputs and outputs, save/delivery, translation into host-owned types, and the session-owned store mapping a placed member ID to its `npc.NPC`/`npc.Data` content (parallel to `SessionData.NPCs` for monster sheets). |

`npc` is not a monster twin. It should not implement combatant interfaces,
carry HP/AC/actions by default, or attach to the event bus. If later work adds
attackable or hostile NPCs, it should do so through explicit policies and likely
new behavior packages rather than by sliding all world NPCs into `monster`.

The reason this package lives outside `rulebooks/dnd5e` is that NPC identity,
display name, capability labels, disposition policy, observation policy, and
blocking defaults are toolkit-level world-entity concepts. D&D-specific code can
decide how those NPCs participate in encounter visibility, movement, session
save/load, and combat exclusion. Another rulebook should be able to reuse the
same definition shape without importing D&D.

In that composition, `npc.NPC` is the generic carrier and D&D vendor types fill
in the vendor-specific data. For example, a D&D blacksmith, general merchant, or
healer can share the same generic NPC identity/capability/policy shape while
their item refs, stock rules, prices, and buy flow remain D&D vendor content.

The generic type should be named `NPC`, not `Definition`. Its reusable nature is
expressed by where it is used: `npc.NPC` describes the common NPC record, while
`encounter.WorldNPC` is the placed runtime instance.

## Encounter Model

Extend the encounter member taxonomy with a world member kind:

```go
const KindWorld MemberKind = "world"
```

The encounter kind names the runtime bucket. It carries no reference to NPC
content — `encounter` does not import `npc`, does not know a merchant from a
villager, and does not carry a ref, capability list, or policy word for
`KindWorld` any more than it carries them for `KindMonster` today (2026-09-02
amendment). A `KindWorld` `MemberInput`/`JoinInput` uses the exact same fields
every kind already has:

- `ID`, `Kind` (`KindWorld`), `Name`, `Position`/`Cell` — required, same as
  any member.
- `SpeedFeet`, `SightFeet`, `Actions`, `Targeting` — zero/empty by caller
  choice for a stationary, non-acting NPC. `encounter` does not enforce this;
  it never has for kind-appropriate field combinations beyond the one rule
  below, and a `KindWorld` member is no exception.
- `Decider` — forbidden, exactly mirroring the one existing kind-specific
  rule this package already enforces (`KindPlayer` + `Decider != nil` →
  reject, design law C2): `KindWorld` + `Decider != nil` → reject, same
  error, same shape.

That is the entire encounter-side model. No `Ref`, no `Capabilities`, no
`CombatPolicy`, `ObservationPolicy`, `MovementPolicy`, or `DispositionPolicy`
field exists on any encounter type. `npc.NPC`'s ref and policies are real and
already shipped (`npc/v0.1.0`), but they are consumed at the `session` layer
only — see Session Seam.

Being non-combatant is not a policy `encounter` reads; it is a structural
consequence of `Kind == KindWorld` never appearing in `sidesInContactOrder`'s
switch, in `Pump`'s `KindMonster` filter, or in any turn-clock transfer path
— see Combat Exclusion. There is nothing to default, because there is nothing
to carry.

## Relationship to #1275

#1275 owns vendor and NPC inventory: durable item refs, finite/infinite stock,
generated stock through `tools/selectables`, quote-before-buy, and buy-only item
transfer into character inventory.

#1280 owns the framework that lets a vendor exist in the world at all:

- identity;
- common NPC definition/profile data;
- placement on the encounter canvas;
- optional movement blocking;
- learned location intel;
- interaction capability reporting;
- non-combat/non-hostile policy.

The two issues compose this way:

1. #1280 answers "there is an interactable NPC here, and it exposes the vendor
   capability."
2. #1275 answers "given that vendor, what stock can be quoted and bought?"

This issue must not implement stock, quote resolution, purchase, currency, or
inventory mutation. It should expose enough descriptor data for #1275 or the
host seam to route into those systems later.

When #1275 adds vendor types, those types should compose with `npc.NPC` records
instead of redefining NPC identity, placement, visibility, blocking, combat
policy, or interaction descriptors.

## First Vendor Profile

The first concrete user of this foundation is the #1275 vendor-like world NPC.
It still uses the generic world NPC model; it is not itself a shop
implementation.

Profile, held entirely at the `session` layer (2026-09-02 amendment — none of
this crosses into `encounter`):

- `Ref` uses `*core.Ref` and identifies the common NPC/vendor profile, stored
  in session's member-ID-keyed NPC store.
- `DisplayName` gives the player-facing name — also carried as the bare
  `Name` fact on the `encounter` member itself, same as a monster's name is.
- `Capabilities` includes `npc.CapabilityVendor`.
- `CombatPolicy` is `npc.CombatPolicyNonCombatant`.
- It never counts as an ally, enemy, prey, or combat side for either players or
  monsters in the MVP — guaranteed structurally by `KindWorld`, not by this
  policy word.

This profile exists to force the generic model through a real lane: a player can
walk up to the vendor, get a descriptor, see the vendor capability, and then a
nearby monster can act without treating the vendor as a target or combatant.
Buying, selling, inventory, prices, and quote flow remain #1275 work.

## Read Shape

`Member` reports the member kind and name, as it does today, for every kind
including `KindWorld` — nothing more. It does not carry NPC capabilities or
policy; those are not encounter facts (2026-09-02 amendment).

The interaction descriptor a player-facing call returns is assembled at the
`session` layer, by looking up the target member ID in session's own NPC
store — exactly parallel to how `session.Attack` reads a monster's sheet out
of `SessionData.NPCs` by member ID rather than asking `encounter` what kind
of monster it placed:

```go
// session-owned type
type WorldNPCDescriptor struct {
    TargetID     MemberID
    Ref          *core.Ref
    DisplayName  string
    Capabilities []npc.Capability
    CombatPolicy npc.CombatPolicy
}
```

The descriptor is copy-out: mutating the returned slice does not mutate
session's stored record.

## Interaction Verb

`encounter` owns only what it already knows how to answer for any member —
identity, adjacency, and visibility. Add an encounter verb scoped to exactly
that:

```go
type InteractInput struct {
    Actor  MemberID
    Target MemberID
    Range  int // optional; zero means adjacent / one cell
}

type InteractOutput struct {
    Target MemberID // confirms identity; session resolves content from this
    Seq    uint64
}
```

Semantics:

- nil input rejects with `ErrNilInput`;
- empty actor or target rejects with `ErrNoMember`;
- closed encounter rejects with `ErrClosed`;
- actor must exist and be a player member in the MVP;
- target must exist and be a `KindWorld` member;
- actor and target must both be placed on the canvas;
- target must be within the configured range, defaulting to adjacent;
- target must be visible to the actor at interaction time;
- the verb appends a story beat and returns confirmation of who was reached;
- it does not change NPC state or execute feature-specific behavior, and it
  has no descriptor to build — that assembly happens one layer up.

`session`'s own `Interact` verb calls this, then looks up `Target` in its NPC
store to build the actual `WorldNPCDescriptor` the player sees. The plan may
split read-only descriptor lookup from beat-writing interaction if that fits
existing session event delivery better. The MVP acceptance needs a
player-facing interaction call that a host can wire.

## Combat Exclusion

Every combat entry point must prove world NPCs do not leak in:

- contact classification ignores world NPCs because they are on neither the
  player nor monster side;
- `Pump` does not consult deciders for world NPCs;
- fight `Form` and straggler joins never include world NPCs;
- `ClockOf` for a world NPC remains world-clock only — automatic, not built:
  `e.clock.Join(...)` runs unconditionally for every member kind in both
  `NewEncounter` and `Join` (R6, "every member is on exactly one clock"), and
  the world clock is the only one a `KindWorld` member can ever be
  transferred onto (`Transfer` guards `ClockTurn` against it directly). The
  world clock is also what makes the NPC discoverable and interactable at
  all: `Pump`'s and `Join`'s sight refresh runs for every present member,
  `KindWorld` included, with no kind filter — that shared, kind-agnostic
  path is how a player ever perceives or later interacts with one, not a
  separate mechanism this issue adds;
- `TurnDriver` is never asked for a world NPC;
- `Afford` exposes no attack target candidate rows for world NPCs;
- `Attack` rejects a world NPC target before resolution;
- monster target selection ignores world NPCs because they are not player
  opponents and not valid attack candidates.

Sight intel about an NPC is not combat contact. A player first-contacting a
world NPC, an observer-capable NPC first-contacting a player, or two NPCs seeing
each other must not form or join a fight bubble.

Hostility is not inferred from visibility, capability, or member kind.
**Correction (2026-09-01):** for `KindWorld` specifically, this needs no new
mechanism at all. `sidesInContactOrder`'s `switch member.Kind` has no
`default` case — a member typed neither `KindPlayer` nor `KindMonster`
already falls into neither slice and never enters `classify()`'s `engaged`
set. `KindWorld`'s neutrality is free, today, with zero changes to that
function. Implementation should add a regression test proving this, not a
`case KindWorld:` arm (there is nothing to write).

The graph-relation amendment above still describes the right mechanism for
the day something needs a *non-neutral* disposition — a hostile or allied
`KindWorld` NPC, or a `KindMonster` that isn't unconditionally opposing every
player. Both are this doc's own listed non-goals, so that day is not this
issue's. The amendment is recorded as forward context for whichever later
idea takes those non-goals on; it is not a prerequisite for shipping
`KindWorld` here, and Task 3/4 should not build it.

## Placement and Blocking

World NPC placement uses the same floor checks as any member — no new rule.

MVP placement rules:

- valid non-empty ID;
- valid `KindWorld` member (no decider, per the Encounter Model section);
- integral dungeon cell;
- cell is owned by a region;
- cell is not blocked by movement-blocking terrain, a wall crossing, or a
  prop — the same checks any member's placement already passes through;
- placing the NPC never creates a fight bubble or initiative turn.

Movement blocking between members is out of scope for this slice (2026-09-02
amendment): `memberEntity.BlocksMovement()` returns `false` for every member
today, and a `KindWorld` member gets the same answer a monster already does.
Nothing here claims a blocking NPC — that is real future work, done
deliberately, not assumed.

## Persistence

`EncounterData.MemberData` needs nothing new for `KindWorld` beyond the kind
value itself — the same fields every member already persists (`ID`, `Kind`,
`Name`, `Cell`, `SpeedFeet`, `SightFeet`, `Actions`, `Targeting`). No ref, no
capabilities, no policy word lives in `EncounterData` (2026-09-02 amendment).

Required load behavior:

- blobs without a `"world"` kind continue loading existing player/monster
  members unchanged;
- a `KindWorld` blob loads with the same validation as Setup/Join (no
  decider) and nothing more to reject.

The placed `npc.NPC`/`npc.Data` content lives in `session`'s own persisted
store, keyed by member ID, parallel to `SessionData.NPCs` for monster sheets
but under a new field name — `SessionData.NPCs` already means spawned
monster sheets, and this must not collide with it (N1). If #1275 adds NPC
inventory persistence, it should reference or compose with this store rather
than redefining placement or interaction.

## Session Seam

`session` is where an NPC actually becomes an NPC (2026-09-02 amendment). It
owns:

- a member-ID-keyed store of placed `npc.NPC`/`npc.Data` content, parallel to
  `SessionData.NPCs` for monster sheets, under a new field name (N1: must not
  collide with the existing meaning);
- a placement verb mirroring `Spawn`'s shape exactly: resolve the `npc.Ref`
  into content, record it in the new store, call `place()` →
  `encounter.Join` with `KindWorld` and the bare facts, same order-of-writes
  reasoning `Spawn` already documents (sheet recorded before placement, so a
  sight refresh that reads back mid-verb has something to find);
- an `Interact` verb: load the encounter, call `encounter.Interact` to
  confirm identity/adjacency/visibility, look up the confirmed target ID in
  the NPC store to build the `WorldNPCDescriptor`, save the changed encounter
  if a beat was written, publish resulting events, and return the
  host-shaped descriptor;
- projecting `WorldNPCDescriptor`s and event beats.

Names should avoid the current `NPC` ambiguity in `SpawnOutput.NPC`, which
means monster state. Prefer `WorldNPC` for the placed record and
`WorldNPCDescriptor` for the interaction read.

## Acceptance Criteria

- `encounter` gains `KindWorld` and places it with the same bare member facts
  a monster gets — no `npc` import, no ref/capability/policy field anywhere
  in `encounter`'s types.
- `encounter` rejects a `KindWorld` member carrying a `Decider`, mirroring the
  existing player-decider rejection exactly.
- World NPCs can be placed on valid dungeon cells, at Setup and mid-session
  (`Join`), and persisted/loaded with no new required fields.
- An encounter with no `KindWorld` member behaves byte-identically to one
  built before this feature existed.
- World NPC placement never adds the NPC to a fight bubble or initiative;
  `Pump` never asks it for a decision; contact classification never puts it
  on either side — proven by tests against the public API, not asserted from
  a code comment.
- `session` owns a member-ID-keyed store of placed NPC content (`npc.NPC`/
  `npc.Data`), under a field name distinct from `SessionData.NPCs`.
- A nearby player can interact with a world NPC through `session.Interact`
  and receive a `WorldNPCDescriptor` assembled from that store.
- Interaction requires adjacency plus current visibility, checked by
  `encounter`; a distant or non-visible target is refused.
- Session-level attack/targeting flows never expose a `KindWorld` member as a
  candidate.
- The first vendor profile (non-hostile, `npc.CapabilityVendor`) exercises
  the whole lane: placed, interacted with, ignored by a nearby fight.
- Tests cover placement (Setup and Join), persistence round-trip, combat
  exclusion, and the interaction lane end to end.
