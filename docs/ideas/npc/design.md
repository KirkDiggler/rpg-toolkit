# NPC Package Specification

**Status:** PROPOSED
**Issue:** [rpg-toolkit#1280](https://github.com/KirkDiggler/rpg-toolkit/issues/1280)
**Module:** `github.com/KirkDiggler/rpg-toolkit/npc`
**Related:** [World NPCs](../world-npcs/),
[rpg-toolkit#1275](https://github.com/KirkDiggler/rpg-toolkit/issues/1275)

## Purpose

Create a top-level `npc` package that defines the generic toolkit NPC content
record.

An NPC is a reusable content entity: a named person-like world object with a
stable toolkit ref, interaction capability labels, default physical presence,
and broad policies that other systems may interpret.

This package intentionally does not place an NPC in an encounter, run a living
world, decide hostility, execute shop behavior, mutate inventory, or model D&D
rules. It gives those systems a shared content shape to compose with.

## Required Package Boundary

`npc` is rulebook-agnostic and lives at the repository root, beside packages
such as `items` and `world`.

Allowed direct imports:

- `github.com/KirkDiggler/rpg-toolkit/core`;
- standard library packages needed for errors, validation, and copying.

Forbidden imports:

- `github.com/KirkDiggler/rpg-toolkit/world`;
- `github.com/KirkDiggler/rpg-toolkit/rulebooks/...`;
- `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter`;
- `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session`;
- D&D vendor, monster, character, combat, or resolution packages.

The package must be usable by any rulebook or host without importing D&D.

## Public Contract

The package exports `NPC` as the primary type. Do not name the main reusable
record `Definition`.

```go
type NPC struct {
    ref               *core.Ref
    displayName       string
    capabilities      []Capability
    combatPolicy      CombatPolicy
    observationPolicy ObservationPolicy
    dispositionPolicy DispositionPolicy
    movementPolicy    MovementPolicy
}
```

The exact storage fields may be private if that matches local package style.
The public read surface must expose equivalent facts:

```go
func New(config Config) (*NPC, error)

func (n *NPC) Ref() *core.Ref
func (n *NPC) DisplayName() string
func (n *NPC) Capabilities() []Capability
func (n *NPC) CombatPolicy() CombatPolicy
func (n *NPC) ObservationPolicy() ObservationPolicy
func (n *NPC) DispositionPolicy() DispositionPolicy
func (n *NPC) MovementPolicy() MovementPolicy
func (n *NPC) ToData() *Data

func (p MovementPolicy) BlocksMovement() (bool, error)
```

`New(Config)` follows the local content-object shape used by packages such as
`monster`, while returning an error because the generic NPC package has a small,
strict validation contract.

`Ref()` should follow existing toolkit ref conventions and return `*core.Ref`.
`core.Ref` is treated as an immutable identifier by convention. Slice copy-out
behavior is mandatory.

## Data Contract

`Data` is the serializable form.

```go
type Config struct {
    Ref               *core.Ref
    DisplayName       string
    Capabilities      []Capability
    CombatPolicy      CombatPolicy
    ObservationPolicy ObservationPolicy
    DispositionPolicy DispositionPolicy
    MovementPolicy    MovementPolicy
}

type Data struct {
    Ref               *core.Ref         `json:"ref,omitempty"`
    DisplayName       string            `json:"display_name"`
    Capabilities      []Capability      `json:"capabilities,omitempty"`
    CombatPolicy      CombatPolicy      `json:"combat_policy"`
    ObservationPolicy ObservationPolicy `json:"observation_policy"`
    DispositionPolicy DispositionPolicy `json:"disposition_policy"`
    MovementPolicy    MovementPolicy    `json:"movement_policy"`
}
```

The loader contract is:

```go
func Load(data *Data) (*NPC, error)
```

This intentionally mirrors the pointer-`Data` shape used by nearby packages.
No `context.Context` is required unless implementation discovers a local
package convention that requires it; unlike `monster`, `npc` has no event bus or
runtime attachment work to cancel.

`ToData` returns a deep copy. Mutating `Data.Capabilities` returned from
`ToData` must not mutate the source `NPC`.

`Load` must copy incoming slices. Mutating the caller's `Data.Capabilities`
after `Load` must not mutate the loaded `NPC`.

## Field Semantics

| Field | Required | Owner | Meaning |
|---|---:|---|---|
| `Ref` | yes | `npc` | Stable toolkit content reference. Uses `*core.Ref`, not a plain string. |
| `DisplayName` | yes | `npc` | Player-facing name, such as `Merchant`. |
| `Capabilities` | no | `npc` | Opaque labels that other systems may route on. |
| `CombatPolicy` | yes | `npc` | Authored combat participation policy. MVP value is non-combatant. |
| `ObservationPolicy` | yes | `npc` | Whether the NPC is only observed or can also observe. |
| `DispositionPolicy` | yes | `npc` | Authored default stance. Not a hostility/team model. |
| `MovementPolicy` | yes | `npc` | Authored/default movement-occupancy policy. Runtime may copy and then mutate its own current movement blocking answer. |

## Capabilities

Capabilities are labels, not executable behavior.

```go
type Capability string

const (
    CapabilityVendor Capability = "vendor"
)
```

`vendor` is the only built-in capability in this slice.

Unknown capability strings are allowed. They must load, copy, and round-trip
unchanged. This lets future rulebooks and host systems carry new labels without
requiring the generic package to know about them first.

The package must not define speculative constants for talk, trainer,
quest-giver, quest-target, dialogue, services, factions, or reputation until a
real consumer lands.

## Policies

Policies are validated names because they imply behavior that consumers may
depend on.

```go
type CombatPolicy string

const (
    CombatPolicyNonCombatant CombatPolicy = "non_combatant"
)

type ObservationPolicy string

const (
    ObservationPolicySubjectOnly ObservationPolicy = "subject_only"
    ObservationPolicyObserver    ObservationPolicy = "observer"
)

type DispositionPolicy string

const (
    DispositionPolicyNeutral DispositionPolicy = "neutral"
)

type MovementPolicy string

const (
    MovementPolicyBlocking MovementPolicy = "blocking"
    MovementPolicyPassable MovementPolicy = "passable"
)
```

`CombatPolicyNonCombatant` means the NPC content is authored as a non-combatant
by default. It does not by itself prove encounter behavior; encounter/session
must still enforce their own combat rules.

`ObservationPolicySubjectOnly` means a runtime may treat the NPC as something
others can observe, but should not give that NPC its own observations from this
content alone.

`ObservationPolicyObserver` means a runtime may give the NPC its own observation
state if that runtime supports it.

`DispositionPolicyNeutral` is a default stance, not an allegiance engine.
Pairwise hostility, teams, factions, charm, fear, hirelings, guards, or
pickpocket consequences belong to runtime/world/encounter systems.

**Cross-reference (2026-09-01):** `../world-npcs/design.md`'s amendment names
the mechanism those runtime systems should use — `world/graph` `Relation`
edges (`HostileTo`/`AlliedWith`), folded per-question, not a widened policy
enum here or in `encounter`. This package's `DispositionPolicy` stays exactly
what it already is: the authoring default a placement seeds from, nothing
more.

## Runtime Defaulting

`MovementPolicy` is part of generic NPC content because physical presence is not
D&D-specific. For the first vendor-like NPC, the authored/default value should
normally be `MovementPolicyBlocking`.

This field is a named policy instead of a bool on purpose. The current
`tools/spatial` cell-occupancy seam asks an existing occupant only
`BlocksMovement() bool`, so today's encounter adapter can only map NPC movement
policy to a binary answer:

- `MovementPolicyBlocking` maps to `BlocksMovement() == true`.
- `MovementPolicyPassable` maps to `BlocksMovement() == false`.

That is the current adapter limit, not the generic NPC concept. Other spatial
seams already pass the moving entity into passability checks, for example
room-to-room connections use `IsPassable(entity core.Entity) bool`. Future cell
occupancy may need the same shape for policies such as "blocks enemies, lets
allies pass." Naming this field `MovementPolicy` preserves that path without
teaching `npc` about teams, factions, or hostility today.

`MovementPolicy.BlocksMovement() (bool, error)` is the sanctioned helper for
the current binary adapter seam. It must map only policies that have a truthful
binary answer. Future policies that need mover-vs-occupant context should return
an error from this helper until a richer adapter exists.

The value is still only a default for runtime placement. Once an NPC is placed
in an encounter or represented in a living world, the owning runtime must store
its current state explicitly. Changing a reusable `npc.NPC` profile later must
not silently rewrite an already-loaded scene.

Examples of runtime-owned changes:

- an NPC moves;
- an NPC is removed from the map;
- an NPC becomes non-blocking;
- an NPC becomes hostile through a world event;
- an NPC gains or loses observation state.

None of those changes are written back into `npc` unless a caller deliberately
edits the content record.

## Validation

`Load` must reject malformed policy/data and accept opaque capabilities.

| Case | Result |
|---|---|
| nil `Ref` | reject |
| malformed `core.Ref` | reject if `core.Ref` can represent malformed data |
| empty `DisplayName` | reject |
| nil or empty `Capabilities` | accept |
| unknown capability | accept and preserve |
| empty `CombatPolicy` | reject |
| unknown `CombatPolicy` | reject |
| empty `ObservationPolicy` | reject |
| unknown `ObservationPolicy` | reject |
| empty `DispositionPolicy` | reject |
| unknown `DispositionPolicy` | reject |
| empty `MovementPolicy` | reject |
| unknown `MovementPolicy` | reject |
| `MovementPolicyPassable` | accept and preserve |

Errors should use package sentinels compatible with `errors.Is`, following
nearby toolkit package style.

## Non-Goals

`npc` must not implement:

- encounter placement;
- map position, facing, region, or visibility state;
- living-world graph/entity projection;
- `world.Scenario` construction;
- vendor stock;
- quote or purchase flow;
- wallets, currency, or inventory mutation;
- D&D stats;
- HP, AC, attacks, actions, turn drivers, or conditions;
- pairwise hostility, teams, factions, or allegiance;
- client art/model selection.

## Integration Contracts

### D&D Vendors

D&D vendor content may compose with `npc.NPC`.

```text
npc.NPC:
  ref: dnd5e:npcs:merchant
  display name: Merchant
  capabilities: [vendor]
  combat policy: non_combatant
  observation policy: subject_only or observer
  disposition policy: neutral
  movement policy: blocking

D&D vendor content:
  item refs
  stock rules
  price rules
  quote/buy hooks
```

The D&D package owns D&D item refs, stock, price assumptions, quote flow, buy
flow, and inventory mutation. The generic package owns only the shared NPC
record.

### Living World

The top-level `world` module owns graph entities, relations, witnessed facts,
verbs, quests, and goals.

`npc` must not import `world`. A content or adapter package may project
`npc.NPC` into a `world.Scenario` by declaring graph entities, relations, slots,
and verbs. That projection is outside this package because different games may
project the same NPC content differently.

### Encounter

The current live-play encounter stack lives under
`rulebooks/dnd5e/encounter`. It may consume or mirror `npc.NPC` data when
placing an NPC on the map.

Placed encounter facts are runtime facts:

- member ID;
- member kind, possibly `KindWorld`;
- cell/region/facing;
- current movement blocking derived from or copied from `MovementPolicy`;
- current visibility or location knowledge;
- current combat membership;
- current hostility/team/allegiance.

Those facts do not belong in `npc`.

If the encounter member kind is named `KindWorld`, that is compatible with this
spec. `npc` names the content bucket; `KindWorld` names how the placed member
belongs to the encounter.

## Acceptance Criteria

- `npc` exists as a top-level module/package.
- `npc` imports `core` and no runtime/rulebook packages.
- The primary public type is `NPC`.
- `NPC` uses `*core.Ref`.
- `Data` carries `*core.Ref`, display name, capabilities, policies, and
  `MovementPolicy`.
- `Load` rejects missing/unknown policy values.
- `Load` accepts unknown capability strings.
- `MovementPolicyPassable` survives load and round-trip.
- `MovementPolicy.BlocksMovement()` maps `blocking` to true and `passable` to
  false.
- `MovementPolicy.BlocksMovement()` rejects missing/unknown movement policies.
- `ToData` and all slice accessors are copy-out.
- Tests prove caller mutation of input/output capability slices cannot mutate an
  `NPC`.
- Package docs state that vendor behavior, living-world projection, and
  encounter placement live outside `npc`.
