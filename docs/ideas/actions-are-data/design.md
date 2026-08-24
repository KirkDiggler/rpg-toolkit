# Actions Are Data; Resolution Machines Interpret Shared Profiles

**Status:** Approved — module-isolated implementation delivery open

**Issue:** [rpg-toolkit#1198](https://github.com/KirkDiggler/rpg-toolkit/issues/1198)

**Decision:** [ADR-0045](../../adr/0045-actions-are-data.md)

**Execution:** [plan.md](plan.md)

This is the live implementation design. PR #1214 remains open while #1198 is
implemented so discoveries can be reconciled here rather than hidden in code.
If implementation requires an architectural deviation, update this design and
obtain Kirk's approval before continuing. Add `implementation.md` only after
all module implementation PRs merge; that file records observed nuances and
final evidence.

## Context

D&D content is intended to be data-driven. A monster should describe what it
can do by filling in rulebook-owned forms. A character may derive the same
forms from its sheet, equipment, and choices. Resolution should interpret the
result without knowing whether a monster, character, spell, item, or feature
produced it.

The current attack path violates that direction. `resolution.AttackProfile` is
defined beside the Strike machine, while `resolution.AttackFromMonsterAction`
switches on monster-action refs, calls the monster action loader, discards the
loaded action, decodes the action-specific config again, and copies its fields
into the profile. Bite-specific facts such as its fixed name, reach, save gate,
and prone consequence therefore live partly in monster content and partly in
resolution. Adding another attack-shaped action requires resolution code even
when its behavior is already expressible by the existing Strike steps.

Moving the existing type directly into monster actions would fix the local
switch but preserve the wrong ownership. Character, spell, and environmental
attacks would then depend on monster vocabulary. Moving it unchanged into
resolution's public surface does not solve the dependency problem either:
resolution imports the D&D rulebook packages needed to load participants and
attach their effects, so those packages cannot import resolution without a
cycle.

The executable action model recorded in ADR-0021 and ADR-0028 points in the
same wrong direction. It gives action objects `Activate`, `Apply`, `Remove`,
subscriptions, and self-removing lifecycles. Those are effect responsibilities.
In the current architecture, ongoing behavior is a condition: it owns its
subscriptions and lifecycle. An action is the declaration of something an
actor may attempt; it does not run itself.

Three accepted decisions constrain the replacement:

- ADR-0038: resolution owns the one interaction bus and drives machines over
  data. Machines yield steps and never own the bus.
- ADR-0039: contestability is data. A `SaveGate` declares how a consequence can
  be resisted; resolution turns it into a save request.
- ADR-0041: attack damage is an ordered collection of typed pools, folded once
  and applied once by resolution.

The missing decision is where action definitions and machine-input profiles
live, which layer authors them, and which layer interprets them.

## Decision

### Actions are inert, self-describing data

An action is a serializable definition. It has no behavior, event bus,
subscriptions, lifecycle methods, or knowledge of the machine that will
interpret it.

The common envelope owns identity, presentation, cost, and exactly one typed
profile:

```go
type Definition struct {
    Ref  core.Ref             `json:"ref"`
    Name string               `json:"name"`
    Cost *combat.SpendProfile `json:"cost,omitempty"`

    // Exactly one profile is present.
    Attack *AttackProfile `json:"attack,omitempty"`

    // Added only when their machines exist.
    SaveArea *SaveAreaProfile `json:"save_area,omitempty"`
    Healing  *HealingProfile  `json:"healing,omitempty"`
    Sequence *SequenceProfile `json:"sequence,omitempty"`
}
```

The concrete field names beyond `Attack` are illustrative future additions,
not scaffolding to build now. The rule is the stable part: a definition carries
one typed profile, not a machine name, executable interface, or opaque
`kind + json.RawMessage` pair.

A profile names a rules interaction independently of its implementation. An
`AttackProfile` means "resolve an attack roll against a target's defense." The
Strike machine consumes that form because Strike implements that interaction;
the form does not know Strike exists. A fireball or dragon breath does not have
an `AttackProfile` because it makes no attack roll. Its different step sequence
requires a different profile and machine.

### Shared contracts live in `combat/actions`

The forms live in the package:

```text
rulebooks/dnd5e/combat/actions
```

This is a package inside the existing `rulebooks/dnd5e` Go module, not a new Go
module. A new module would add independent tags and dependency pins without
breaking a dependency cycle that a package boundary already breaks.

`combat/actions` is a strict data-contract leaf. It may import lower rulebook
vocabulary such as core refs, abilities, damage, and saves. It must not import
monster, character, resolution, or event-bus packages.

The dependency direction is:

```text
monster content -----\
character assembly ---+--> combat/actions <-- resolution
spell content --------/
```

No producer depends on a consumer, and no consumer owns a producer's schema.

### `AttackProfile` describes any attack-roll interaction

An attack profile covers melee and ranged weapon attacks, natural and unarmed
attacks, and melee and ranged spell attacks. It does not cover saving-throw-only
interactions.

```go
type AttackProfile struct {
    Category    AttackCategory `json:"category"`
    Delivery    AttackDelivery `json:"delivery"`
    AttackBonus int            `json:"attack_bonus"`

    // Optional evidence. Precomputed stat blocks may honestly omit it.
    Ability *AbilityContribution `json:"ability,omitempty"`

    // Optional wielded-weapon evidence. A natural or unarmed weapon attack
    // need not carry it.
    Weapon *WeaponContext `json:"weapon,omitempty"`

    Damage []damage.Damage        `json:"damage,omitempty"`
    OnHit  []ConditionApplication `json:"on_hit,omitempty"`
}

type AbilityContribution struct {
    Ability  abilities.Ability `json:"ability"`
    Modifier int               `json:"modifier"`
}

type WeaponContext struct {
    Ref              *core.Ref `json:"ref,omitempty"`
    TwoHanded        bool      `json:"two_handed"`
    OffHandWeaponRef *core.Ref `json:"off_hand_weapon_ref,omitempty"`
}
```

`AttackCategory` distinguishes weapon and spell attacks. Delivery is a tagged
union rather than a boolean plus fields that are meaningless half the time:

```go
type AttackDelivery struct {
    Melee  *MeleeDelivery  `json:"melee,omitempty"`
    Ranged *RangedDelivery `json:"ranged,omitempty"`
}

type MeleeDelivery struct {
    ReachFeet int `json:"reach_feet"`
}

type RangedDelivery struct {
    NormalFeet int `json:"normal_feet"`
    LongFeet   int `json:"long_feet,omitempty"`
}
```

Exactly one delivery arm is present. A zero long range means the attack has no
separate long-range bracket. Delivery kind and distance remain separate facts:
melee is not a proxy for proximity. A reach weapon can attack from ten feet,
and a ranged weapon can attack a creature next to its wielder.

`Ability` carries static evidence for rules that predicate on the ability used.
Rage, for example, requires a melee weapon attack using Strength. Character
assembly supplies the selected ability and modifier. A monster stat block with
precomputed attack and damage numbers may leave `Ability` nil rather than guess
which arithmetic produced them.

Damage remains ADR-0041's ordered typed pools. Damage may be empty when an
attack has another meaningful on-hit result, such as a net-like attack that
applies a condition without dealing damage.

### Conditions are the executable effect boundary

An attack declares on-hit conditions as data:

```go
type ConditionApplication struct {
    Ref        core.Ref        `json:"ref"`
    Parameters json.RawMessage `json:"parameters,omitempty"`
    Save       *saves.SaveGate `json:"save,omitempty"`
}
```

`Parameters` are owned and validated by the referenced condition's package.
Neither `combat/actions` nor resolution interprets them. A nil `Save` means the
condition applies automatically on a hit. A non-nil gate means resolution
requests the declared save and applies the condition on failure. A condition
application uses the gate's negation-on-success semantics; save-for-half damage
belongs to a damage or save-area declaration rather than pretending a
condition was halved.

Multiple applications are ordered data. This covers an attack that applies
more than one condition without introducing an effects language. The condition
registry supplies behavior by ref after the declaration is validated. Bite,
prone, paralysis, and every future condition remain unknown to Strike.

The old `resolution.Consequence` behavior interface does not cross this data
boundary. Resolution may use private executable machinery to interpret a
condition declaration on its own bus, but the persisted action carries only
refs, parameters, and gates.

### Producers author or assemble the same definition

Monster factories author shared definitions directly and persist them in each
monster's action list. There is no `BiteConfig -> BiteAction -> AttackProfile`
path. A
bite, claw, slam, shortsword, or shortbow is data unless it introduces a new
rules interaction.

Characters are different producers, not a different contract. A
character-owned assembler derives definitions from the sheet, equipment,
features, and declared choices at the moment actions are requested. Finesse,
proficiency, grip, and equipment context are assembly facts. That assembler
lives with character/rulebook authoring, never in resolution.

Spells and other producers follow the same rule: author or assemble shared
profiles, then hand them over. Producer-specific source data may exist when it
is genuinely the canonical source, but resolution never decodes that source or
switches on its content refs.

### Resolution dispatches profile kinds, never content identities

The selected definition is passed to resolution with runtime intent such as
actor and target IDs. Resolution selects the machine from the one populated
profile arm:

```text
Definition.Attack   -> Strike
Definition.SaveArea -> save-area machine
Definition.Sequence -> sequence machine
```

This is the one appropriate dispatch. It changes when a genuinely new machine
and profile are introduced. It does not change for a new monster, weapon,
spell, attack name, or condition.

Action economy remains above individual machines. The resolution entry path
validates the definition and runtime intent, verifies and charges the declared
cost, then drives the selected machine. Strike knows attack sequencing and
interprets `AttackProfile`; it does not know action catalogs or producers.

### Validation precedes cost, rolls, and mutation

The shared contract validates intrinsic consistency:

- a definition has identity, name, exactly one profile, and a valid cost when one is declared;
- an attack has exactly one delivery arm;
- melee reach and ranged brackets are valid;
- ability and modifier appear together;
- damage pools follow ADR-0041;
- an attack has at least damage or one on-hit condition;
- condition/save pairs are structurally valid.

The condition registry validates refs and condition-owned parameters. Resolution
validates runtime facts: actor and target exist, the action is available to the
actor, the target is legal and in range, and the cost can be paid.

The order is:

```text
validate definition and profile
-> validate referenced condition declarations
-> validate actor, target, availability, and range
-> charge cost
-> roll and execute
```

A malformed definition, unsupported condition, or illegal target consumes no
resource, rolls no die, and mutates nothing. Melee reach and ranged legality
are Strike's interpretation of attack delivery; callers do not reproduce those
rules above the machine.

### Action lifetime is effect state, not action behavior

Actions have no `Activate`, `Apply`, `Remove`, `UsesRemaining`, or event
subscriptions. A temporary opportunity to act is effect or economy state.
Features may apply conditions or grant capacities that cause action assembly to
project additional definitions. Those definitions disappear when their grant
expires or their capacity is consumed. The condition/effect owns the lifecycle;
the action remains inert data.

For example, Flurry of Blows spends Ki and grants the appropriate effect or
capacity. Action assembly then projects the available Flurry attack
definitions. Nothing called `FlurryStrike` subscribes to turn end or removes
itself.

This section supersedes ADR-0021's executable internal action pattern and
ADR-0028's first-class, self-subscribing action objects. It preserves
the useful semantic distinction behind ADR-0028: features grant, actions
declare what may be done, and conditions modify or sustain behavior.

### Migration is a hard cut

The repository will end with one action representation. Implementation may be
sequenced, but no compatibility representation survives on `main`:

- add the pure `combat/actions` contracts;
- migrate monster data and factories to shared definitions;
- move character attack assembly out of resolution;
- make resolution consume and dispatch shared profiles;
- remove `resolution.AttackProfile`, `AttackFromMonsterAction`, and
  `AttackFromCharacter`;
- remove monster action runtime classes that merely restate data;
- census the existing executable `rulebooks/dnd5e/actions` package, migrate
  real effects to conditions/economy, and delete dead or declarative wrappers;
- regenerate pre-alpha fixtures and snapshots rather than translate old action
  JSON at runtime.

No local `replace`, dual wire format, legacy loader, or permanent adapter is
part of the target architecture.

### Delivery follows module boundaries and squash-generated tags

The repository squashes PRs and independently tags each Go module. Therefore a
single PR must not carry the root D&D, encounter, resolution, and session
changes together. That would either require a merge-commit exception or leave
committed pseudo-versions pointing at provider commits rewritten by squash.

Implementation is delivered through one PR per changed Go module. All module
PRs may be opened for concurrent review. While providers are still open, a
consumer PR may temporarily resolve a pushed provider-branch pseudo-version so
its CI can exercise the complete design. A consumer must not merge with that
temporary pin. Providers merge first by squash; the main-branch auto-tag
workflow publishes stable module tags; then each consumer replaces its
temporary pins with those tags, reruns its full gate, and merges by squash.

The dependency order is:

```text
root D&D ────────> resolution ──> session
encounter ──────────────────────> session
```

Root D&D and encounter are independent and may merge first in either order.
Resolution merges only after it pins the released root D&D tag. Session merges
only after it pins the released root D&D, encounter, and resolution tags.
Repository-wide documentation remains on PR #1214 until all code PRs land.

## Consequences

### Positive

- Adding attack-shaped content that uses existing rules requires data, not a
  resolution case or new Go action class.
- Monster, character, spell, and future producers share one attack contract.
- Resolution depends only on interaction data and changes only for new machine
  shapes.
- Action definitions are inspectable before execution for UI, behavior,
  affordability, and tuning.
- Save gates and condition effects remain declarative, while actual ongoing
  behavior stays in conditions.
- Invalid states are rejected before cost, randomness, or mutation.
- The dependency graph has one direction and no new independently versioned
  module.

### Negative

- This is a breaking migration across monster persistence, character assembly,
  resolution, tests, fixtures, and nested module versions.
- The existing action packages and ADR-era runtime model require a deliberate
  usage census; not every old type can be deleted without first relocating any
  real rule it still owns.
- A typed profile union must be extended whenever a genuinely new interaction
  machine is introduced.
- Generic resolution dispatch and strict validation add ceremony compared with
  calling a concrete action object's method.

### Neutral

- Character attacks remain assembled while monster stat-block attacks may be
  authored directly; sharing an output contract does not require sharing a
  producer.
- Conditions remain executable Go behavior. "Actions are data" does not create
  an effects DSL.
- Fireball, dragon breath, healing, movement, and multiattack do not become
  Attack profiles merely because they are actions. Their step sequences decide
  their profile and machine.
- ADR-0038's bus ownership, ADR-0039's save-gate model, and ADR-0041's damage
  rules remain in force. This decision changes ownership and data flow, not
  those mechanics.

## Options considered and rejected

### Keep compilers in resolution but register them by ref

A registry removes a switch statement but leaves resolution responsible for
producer schemas and content identity. Every new action shape still changes the
consumer. Rejected as indirection around the same ownership error.

### Put the profile in `monster/actions`

This moves bite knowledge nearer bite but makes character, spell, and
resolution code depend on monster vocabulary. Rejected because an attack is a
rulebook concept, not a monster concept.

### Keep `AttackProfile` in resolution and let producers import it

The type is adjacent to its first consumer, but the current resolution module
imports the rulebook packages that would need to produce it. The resulting
cycle is evidence of misplaced shared vocabulary. Rejected.

### Create a new Go module for action contracts

An independent module also breaks the dependency direction, but the existing
D&D module already provides a lower shared package boundary. A module adds tags,
pins, and release ordering without buying additional correctness. Rejected in
favor of `combat/actions` inside the D&D module.

### Preserve executable action objects that project profiles

An `AttackProfiler` interface would remove duplicate JSON decoding while
retaining `BiteAction`, `MeleeAction`, and other runtime wrappers. It is a safe
local refactor but preserves behavior where the architecture wants data and
requires Go for ordinary content. Rejected as insufficient.

### Store a machine ref plus opaque JSON

A `machine_ref + json.RawMessage` envelope is open-ended, but gives up compile-
time discoverability and makes malformed producer/consumer pairings a runtime
routing problem. The machine set is intentionally explicit: a new step shape is
an architectural addition. Rejected in favor of an exactly-one typed profile
union.

### Carry old and new action formats through adapters

Adapters reduce the immediate migration but create two canonical sources and
let the executable action model survive indefinitely. This project is
pre-alpha and can regenerate its owned data. Rejected in favor of a hard cut.

## Verification

Implementation of this decision is complete when tests prove:

- shared definitions and profiles round-trip through JSON without behavior
  objects;
- zero or multiple profile arms are rejected;
- invalid delivery, ability, damage, and condition declarations are rejected;
- representative monster, character, unarmed, melee, and ranged attacks produce
  the shared form;
- a synthetic attack carrying an unknown content ref resolves through Strike,
  proving resolution does not switch on content identity;
- save-gated and automatic conditions apply from declarations by ref;
- invalid definitions consume no cost, roll no dice, and mutate nothing;
- `combat/actions` imports no producer, resolution, or event-bus package;
- resolution contains no action-specific config decoder or producer compiler;
- no compatibility action representation remains after the cut.
