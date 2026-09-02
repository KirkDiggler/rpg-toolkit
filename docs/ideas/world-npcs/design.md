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

## Scope

Add first-class world NPC support to the modern D&D 5e live-play stack. This
issue builds the general placed-NPC framework that
[rpg-toolkit#1275](https://github.com/KirkDiggler/rpg-toolkit/issues/1275) can
use for its first vendor/NPC inventory implementation.

A world NPC is a placed, interactable, non-combat encounter entity. It stands on
the same dungeon-absolute canvas as players and monsters, may block movement,
can be seen, can optionally observe, and can be interacted with by a nearby
player. It is not a monster, does not enter fight bubbles, takes no turns, and
is not an attack target in the MVP.

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
- **N5 - movement is authored as policy.** `MovementPolicy` is carried on the
  generic NPC record. The current encounter/spatial adapter may map it to
  `BlocksMovement() bool`, but the generic content model must not inherit that
  binary runtime seam.
- **N6 - capabilities are opaque.** The encounter/session layers carry
  capability words and never implement behavior behind them in this issue.
- **N7 - host seam keeps its own twins.** `session` exposes session-owned NPC
  interaction/read types rather than leaking encounter internals.
- **N8 - NPC locations are learned, not assumed.** A world NPC is a valid
  subject for sight and other location intel, but its location is not globally
  known merely because it exists. Players and other observers learn the NPC's
  location only from authored/loaded intel or from actually perceiving it. If
  `rulebooks/dnd5e/encounter/v0.38.0` is the implementation base, NPC sight
  testimony uses `LocationKnowledge` / `EncodeLocationPayload`, not
  hand-marshalled legacy `SightPayload`.
- **N9 - observing is an NPC policy.** Some world NPCs may observe and hold
  intel; some can only be observed. The generic NPC model carries this
  distinction per NPC or NPC type. The MVP allows NPC-held intel but does not
  build behavior that consumes it.
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
| `rulebooks/dnd5e/encounter` | Runtime placement and D&D live-play behavior: member kind, cell, blocking, location intel subject/observer behavior, interaction range/visibility checks, story beat, and combat exclusion. |
| `rulebooks/dnd5e/session` | Host seam: start/load/place/interact inputs and outputs, save/delivery, and translation into host-owned types. |

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

The encounter kind names the runtime bucket, not the content type. The generic
content package remains `npc`, and D&D content refs may still look like
`dnd5e:npcs:merchant`.

`MemberInput` grows fields for world NPC facts:

```go
type NPC struct {
    Ref               *core.Ref
    DisplayName       string
    Capabilities      []InteractionCapability
    CombatPolicy      NPCCombatPolicy
    ObservationPolicy NPCObservationPolicy
    DispositionPolicy NPCDispositionPolicy
    MovementPolicy    npc.MovementPolicy
}

type InteractionCapability string

const (
    InteractionCapabilityVendor InteractionCapability = "vendor"
)

type NPCCombatPolicy string

const (
    NPCCombatPolicyNonCombatant NPCCombatPolicy = "non_combatant"
)

type NPCObservationPolicy string

const (
    NPCObservationPolicySubjectOnly NPCObservationPolicy = "subject_only"
    NPCObservationPolicyObserver    NPCObservationPolicy = "observer"
)

type NPCDispositionPolicy string

const (
    NPCDispositionPolicyNeutral NPCDispositionPolicy = "neutral"
)
```

Exact names may move during implementation to match package style, but the wire
meaning should not: the generic NPC record uses `*core.Ref`; `vendor` is the only
initial built-in capability; additional capabilities should be easy to add later
without changing the record shape; the only shipped combat policy is
non-combatant; and observation policy decides whether the NPC can only be a
subject of others' intel or also receives its own sight refreshes.
Disposition is deliberately a policy word rather than a boolean so later states
such as hostile, helpful, faction-bound, frightened, charmed, escorted, or
temporarily allied can arrive without changing the shape.

**Amended above:** `NPCDispositionPolicy` is the authoring/default word only —
what edge (if any) gets seeded for this NPC when it is placed. It is not what
`classify()` reads at runtime. The live hostility question is a `world/graph`
relation lookup (`HostileTo`/`AlliedWith`, or none — see the amendment section
above), which is exactly what lets the later states this paragraph already
names arrive as new relations rather than new enum values or a rewritten
switch.

For `KindWorld` NPC members:

- `ID`, `Name`, and `Position` are required.
- `Ref` is required and uses `*core.Ref`.
- `Position` is authored and compiled the same way existing setup member cells
  are compiled.
- `Capabilities` is allowed to be empty.
- `MovementPolicy` defaults to `MovementPolicyBlocking` at the high-level
  construction surface, but persistence records an explicit policy.
- `CombatPolicy` defaults to non-combatant at the high-level construction
  surface, but persistence records an explicit word.
- `ObservationPolicy` defaults at the high-level construction surface, but
  persistence records an explicit word for `KindWorld`.
- `DispositionPolicy` defaults to neutral/non-hostile at the high-level
  construction surface, but persistence records an explicit word for
  `KindWorld`.
- `Decider` is forbidden.
- `SpeedFeet`, `Actions`, and `Targeting` must be zero/empty in the MVP.
- `SightFeet` is meaningful only when the NPC's observation policy makes it an
  observer. Subject-only NPCs do not need sight reach because they never build
  their own percept.

Defaults solve authoring convenience, not runtime truth. For example, the
generic `npc.NPC` shape can say the normal default is blocking movement by
policy, neutral disposition, and non-combatant behavior. Once a specific NPC is placed or
loaded, the encounter/session data should carry the explicit current values so
later profile changes do not silently rewrite an existing scene.

World NPCs are valid location-intel subjects, not automatically visible facts.
A player who can see the NPC receives ordinary sight testimony about the NPC's
location; a player who has not seen or otherwise learned about the NPC does not
get its location for free. On the v0.38.0 encounter shape, sight testimony uses
`LocationKnowledge{State: LocationKnown}` when the NPC is perceived and may use
`LocationKnowledge{State: LocationUnknown}` or existing fading/remembering
behavior when prior knowledge becomes stale, according to the final encounter
rules.

Observer-capable NPCs may hold intel. That is allowed because a future vendor,
escort, trainer, or quest target may need to know who is nearby. The MVP does
not use NPC-held intel to drive decisions, dialogue, targeting, or automation.

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

Profile:

- `Ref` uses `*core.Ref` and identifies the common NPC/vendor profile.
- `DisplayName` gives the player-facing name.
- `Capabilities` includes `InteractionCapabilityVendor`.
- `CombatPolicy` is `NPCCombatPolicyNonCombatant`.
- `ObservationPolicy` may be subject-only or observer, depending on the vendor
  type being authored.
- `DispositionPolicy` is neutral/non-hostile.
- It is non-hostile to players and monsters at the start.
- It never counts as an ally, enemy, prey, or combat side for either players or
  monsters in the MVP.
- Its location may be known at encounter start because the authored scenario or
  loaded state gives the party that knowledge.
- Being known/visible for this profile does not change N8: other world NPCs may
  stay unknown until discovered.

This profile exists to force the generic model through a real lane: a player can
walk up to the vendor, get a descriptor, see the vendor capability, and then a
nearby monster can act without treating the vendor as a target or combatant.
Buying, selling, inventory, prices, and quote flow remain #1275 work.

## Read Shape

`Member` should report enough for hosts to distinguish world NPCs from players
and monsters. It should expose the member kind and name as it does today; NPC
capabilities and combat policy may either ride on `Member` or on a dedicated NPC
query, but the interaction result must not require a caller to know internals.

Add an interaction descriptor:

```go
type InteractionDescriptor struct {
    TargetID     MemberID
    Ref          *core.Ref
    DisplayName  string
    Capabilities []InteractionCapability
    CombatPolicy NPCCombatPolicy
    ObservationPolicy NPCObservationPolicy
    DispositionPolicy NPCDispositionPolicy
}
```

The descriptor is copy-out: mutating the returned slice does not mutate the
encounter.

## Interaction Verb

Add an encounter verb:

```go
type InteractInput struct {
    Actor  MemberID
    Target MemberID
    Range  int // optional; zero means adjacent / one cell
}

type InteractOutput struct {
    Descriptor InteractionDescriptor
    Seq        uint64
}
```

Semantics:

- nil input rejects with `ErrNilInput`;
- empty actor or target rejects with `ErrNoMember`;
- closed encounter rejects with `ErrClosed`;
- actor must exist and be a player member in the MVP;
- target must exist and be a `KindWorld` NPC member;
- actor and target must both be placed on the canvas;
- target must be within the configured range, defaulting to adjacent;
- target must be visible to the actor at interaction time;
- the verb appends a story beat and returns the descriptor;
- it does not change NPC state or execute feature-specific behavior.

The plan may split read-only descriptor lookup from beat-writing interaction if
that fits existing session event delivery better. The MVP acceptance needs a
player-facing interaction call that a host can wire.

## Combat Exclusion

Every combat entry point must prove world NPCs do not leak in:

- contact classification ignores world NPCs because they are on neither the
  player nor monster side;
- `Pump` does not consult deciders for world NPCs;
- fight `Form` and straggler joins never include world NPCs;
- `ClockOf` for a world NPC remains world-clock only;
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

World NPC placement uses the same floor and movement-blocking checks as members.

MVP placement rules:

- valid non-empty ID;
- valid `KindWorld` NPC member;
- integral dungeon cell;
- cell is owned by a region;
- cell is not blocked by movement-blocking terrain, wall crossing, prop, or
  another blocking entity;
- if `MovementPolicy` maps to blocking, later members cannot step onto the
  NPC's cell;
- placing the NPC never creates a fight bubble or initiative turn.

The implementation should verify whether the existing canvas treats all members
as blocking. If so, passable NPC movement policy is a real new requirement and
must be implemented deliberately rather than documented as if it already exists.

## Persistence

`EncounterData.MemberData` grows explicit NPC fields or a nested NPC block.

Required load behavior:

- blobs without NPC fields continue loading existing player/monster members;
- a `KindWorld` NPC blob must include explicit `movement_policy`,
  `combat_policy`, `observation_policy`, and `disposition_policy`;
- unknown combat policy rejects with `ErrInvalidData`;
- unknown observation policy rejects with `ErrInvalidData`;
- capabilities round-trip in stable order and remain opaque strings;
- `ToData` is deterministic and copy-out.

If NPCs are present in persisted `intel.Data`, their sight payloads are valid
only as location knowledge about a subject. Loading must not reject intel merely
because the subject is a world NPC, and must not treat that intel as combat
contact or NPC behavior input unless the NPC's observation policy says it can
observe.

If session owns durable NPC records for host-authored world NPCs, use a new
field name, not `NPCs`, because `SessionData.NPCs` currently means spawned
monster sheets. If #1275 adds NPC inventory persistence, it should reference or
compose with the world-NPC identity from this issue rather than redefining
placement or interaction.

If an `npc` definition/profile exists separately from its placed encounter
member, persistence must keep both facts clear: the placed member stores the
instance ID and current world state, while the definition ref names the reusable
profile. Loading must not rebuild current world state from the profile and
silently erase per-instance changes.

## Session Seam

`session` adds host-shaped twins for:

- starting an encounter with authored world NPCs, if the world authoring input
  reaches session construction;
- placing a world NPC mid-session, only if needed for the issue;
- interacting with a world NPC;
- projecting interaction descriptors and event beats.

Names should avoid the current `NPC` ambiguity in `SpawnOutput.NPC`, which
means monster state. Prefer `WorldNPC` or `Interactable`.

The session interaction verb should load the encounter, call the encounter verb,
save the changed encounter if a beat is written, publish resulting events, and
return a host-shaped descriptor. It should not load character or monster sheets
except as needed for existing standing/sight projection gates.

## Acceptance Criteria

- Toolkit has a first-class world NPC model in the modern encounter/session
  stack.
- Encounter setup/load can store world NPCs separately from monster sheets.
- World NPCs can be placed on valid dungeon cells.
- World NPCs expose interaction capabilities.
- A nearby player can interact with a world NPC and receive a descriptor.
- Interaction requires adjacency plus current visibility.
- A distant player cannot interact.
- A world NPC with `MovementPolicyBlocking` blocks movement through the current
  spatial adapter seam.
- A passable world NPC, if supported by the implementation, allows movement
  through or onto its cell according to the final documented rule.
- World NPC placement and interaction do not add the NPC to a fight bubble.
- World NPCs do not receive turns.
- `Pump` does not ask a world NPC decider or turn driver.
- Monster/player contact detection ignores world NPCs for fight formation.
- `Afford` and `Attack` do not allow attacking non-combatant world NPCs.
- Persistence round-trips world NPC identity, name, position, capabilities,
  blocking, combat policy, observation policy, disposition policy, and ref.
- Players can receive location intel about world NPCs they see or otherwise
  learn about.
- Players do not automatically know every world NPC location.
- The first vendor profile can be known at encounter start through authored or
  loaded intel.
- The first vendor profile is non-hostile to players and monsters.
- The first vendor profile exposes enough descriptor data for #1275 to attach
  vendor inventory without #1280 implementing stock or buying.
- Observer-capable world NPCs may hold intel, while subject-only NPCs do not.
- NPC-held intel is not consumed by any MVP NPC behavior.
- Tests cover placement, interaction, persistence, movement blocking, initiative
  exclusion, targeting exclusion, NPC sight testimony, and observer-policy
  behavior.
