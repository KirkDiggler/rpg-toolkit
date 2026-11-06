# ADR-0048: Composing Equipment-Granted Abilities and Effects

Date: 2026-11-04

## Status

Accepted

**Scope: composition and ownership only.** Accepted by KirkDiggler on
2026-11-05, after inspecting the example and agreeing to this bounded scope.

The Decision section establishes responsibilities that implementations follow.
It does not accept the prototype's exact API, resolver protocol, extraction home,
or production integration plan. Experimental evidence is marked separately below;
acceptance is not implementation completion or a permanent naming/schema freeze.

Consumer slice: [Keep signature combat and loot S1 — Second Wind ring](https://github.com/KirkDiggler/rpg-project/issues/862),
under [#850](https://github.com/KirkDiggler/rpg-project/issues/850).
Existing effects work remains owned by [#703](https://github.com/KirkDiggler/rpg-project/issues/703).

The [layer walkthrough](../ideas/item-effect-composition/README.md) explains how
these responsibilities fit the larger architecture. Its [paper-trail map](../ideas/item-effect-composition/paper-trail.md)
connects the investigation to checked plans and observed implementation; neither
adds a second authority for this decision.

## Context

The keep needs equipment whose abilities work for its NPC owners and can become
usable player abilities after acquisition. The immediate investigation is
smaller: a ring grants Second Wind in a runnable example under `examples/`.
The example should expose the reusable composition, not implement a special
healing rule inside one item or claim the full acquisition journey has shipped.

The decision question is:

> How does equipment grant an ability using reusable activation and effect
> behavior, while keeping the grant, its costs and its lifetime explicit?

### What exists

- [`examples/effects`](../../examples/effects/README.md) is an implemented
  prototype with Rage-shaped test customers. Its [`Composed`](../../examples/effects/composed.go)
  runs an effects machine and attaches the outputs of its current state. Its
  vocabulary is supplied by a consumer; neither generic tool imports a rulebook.
- The [composable-conditions design](https://github.com/KirkDiggler/rpg-toolkit/blob/aa41ff1e65a6c3bd3f591849a33f892f18919449/docs/ideas/composable-conditions/design.md)
  distinguishes **Grant**, **Activation**, and **Condition**. Grant is explicitly
  outside that story. Cost is outside the condition; instantaneous healing is
  not a stored condition. This investigation is a consumer of that work, not its
  replacement.
- [`features.SecondWind`](../../rulebooks/dnd5e/features/second_wind.go) currently
  owns a recoverable resource, refuses owners without positive fighter levels,
  and heals `1d10 + fighter level` by publishing a sourced healing event. It has
  a supported JSON round-trip and rest subscriptions. Copying this object into
  an item would carry those assumptions with it.
- [`healing`](../../rulebooks/dnd5e/healing/healing.go) already provides sourced
  healing arithmetic without requiring a spell or event bus. Its caller owns
  payment/delivery and the recipient owns HP mutation. There is no reason to
  invent a second dice/healing calculator merely to demonstrate an item.
- [`actions.Definition`](../../rulebooks/dnd5e/combat/actions/definition.go)
  currently has Attack, Cast and Sequence arms. Cast can declare immediate
  healing. This does not establish that a non-spell item should masquerade as
  a spell, or that the illustrative standalone Healing arm in ADR-0045 exists.

[ADR-0045](0045-actions-are-data.md) keeps action declarations inert and their
interpretation separate from producers. [ADR-0038](0038-resolution-owns-the-bus.md)
owns interaction wiring. A general composition should preserve these boundaries
rather than teach the host or generic effects tool what this ring means.

## Alternatives considered

### A. Grant the existing feature object directly

Reuse `features.SecondWind` with a new source of ownership. This is the shortest
route to exercising the current class ability, its uses and rest behavior.
However, it carries fighter eligibility, fighter-level scaling, private resource
ownership and executable-feature activation. This option was considered while
fighter-only ring behavior was still open. The operator's item-authored level
requirement below means unmodified reuse cannot supply the ring's intended level;
keep this option as the baseline, not a reason to alter the wearer's class levels.

### B. Compose the grant with reusable activation and effect behavior

**Chosen for composition and ownership (R3), not for a fixed API.** The item grants
an opportunity to use an ability; activation handles its declared cost and
immediate consequence; ongoing effects use the existing effects composition
where their lifetime requires it. A consumer example establishes the required
connections without putting equipment rules into `examples/effects`.

This separates a source of an ability from its implementation and lets classes,
items and later consumers share behavior without requiring identical eligibility
or budgets. It costs an explicit account of identity, availability, resource
ownership, removal and reload. The example must demonstrate those seams rather
than hide them behind a wrapper around today's feature.

### C. Give each item its own executable behavior

A ring-specific activation can express any desired gameplay quickly. It also
repeats healing, resource and lifecycle behavior and makes each new item another
implementation. It does not meet the stated reusable-tool goal; it remains here
to explain why a locally working ring alone would be insufficient evidence.

## Decision

Choose B: equipment composes grants with reusable ability/effect behavior rather
than implementing each item's rules independently. Four boundaries govern that
composition:

1. **The item is a composer.** It connects authored grants, their parameters,
   availability and item-owned usage budgets to reusable behavior. It does not
   duplicate the rulebook's healing, Rage or other ability implementation.
2. **Item-specific pools belong to the instance.** Their mutable state is distinct
   from class-granted and other-item pools and remains with the item through
   transfer and supported reload. A content/recipe ref is not an instance ID.
3. **Grants are inspectable data; the consumer owns meaning.** The rules consumer
   validates/interprets the ability and owns effect-specific behavior and delivery.
   Each cost and resulting mutation has one owner, not two payment/application paths.
4. **Representations have distinct responsibilities.** Authored configuration,
   live mutable state, saved irreducible state and derived views are not competing
   mutation authorities. Returning a view or saved snapshot does not expose the
   live item's mutable state.

These boundaries—not the prototype's type names or generic function signatures—
are the accepted architectural decision. Unmodified class-feature reuse (A) does
not satisfy the ring's parameter/ownership requirements; per-item behavior copies
(C) do not satisfy the reusable-tool goal.

The responsibility separation is:

```text
ring / other source
        → grant makes an ability available
        → activation validates and pays its declared cost
        → immediate consequence and/or ongoing effect
```

These are responsibility boundaries, not four required new packages. Sharing
underlying behavior does not mean that an immediate heal must become a persistent
condition or that every ability needs a state machine.

The example remains under `examples/`. Its candidate interfaces may change as
consumer evidence teaches a better shape; an example API is not a production
contract or a selected extraction home. The current cut's [experiment plan](https://github.com/KirkDiggler/rpg-toolkit/blob/probe/862-items/examples/items/PLAN.md)
names its responsibilities, tests and explicit limits. Reuse the existing effects prototype
where applicable, preserving its generic import seam. Finding that the ring
alone does not exercise an ongoing effect is evidence to record, not a reason
to invent one. A contrasting consumer may then be proposed to test that seam.

### Settled use-case parameter

**R1 — Ring-authored Second Wind level.** Settled by KirkDiggler, 2026-11-04.
The ring supplies the level for its granted Second Wind, rather than requiring
that level from the wearer's Fighter class. Level-1 and level-3 rings are concrete
examples: their granted uses heal `1d10 + 1` and `1d10 + 3`, respectively.
The requirement is scoped to the granted ability; it does not authorize changing
the wearer's class levels or overwriting a class-granted Second Wind's scaling.
A wearer with no fighter levels can receive the ring-supplied level. This settles
the level source, not unrelated eligibility/attunement rules.

The operator suggested rarity might determine the item's level. That remains a
possibility, not an accepted rarity-to-level mapping or a requirement to implement
rarity now. Level 1 and level 3 are examples, not a declared exhaustive range.

**R2 — Charges belong to the item instance.** Settled by KirkDiggler, 2026-11-04.
The ring's use pool is separate from class-granted uses and other item instances.
Spending it does not spend the wearer's class pool. The pool follows the ring:
transfer or re-equipping does not refill it. Remaining uses must survive the
item's supported save/load round-trip. The examples of a Raging ring and a
Reckless Attack hotdog motivate reuse; their detailed gameplay is not selected.
Whether several abilities on one item share that item's pool remains open.

**R3 — The item is a composer.** Settled direction from KirkDiggler, 2026-11-04.
An item composes underlying reusable effect behavior with orchestration of usage
and resource pools, analogous to a condition composing effects for its own
lifetime. It is not merely metadata pointing at an independently budgeted class
feature, nor a new copy of healing/Rage behavior per item. The composition supplies
parameters and connects access, its item-owned budget and the resulting behavior.
This assigns responsibilities; it does not choose an executable item interface,
a second payment path or a common package for every kind of composer. The action
and resolution boundaries described above still constrain the implementation.

**R4 — Normal recovery, authored as configuration.** Settled by KirkDiggler,
2026-11-04. The first Second Wind ring restores its own pool on a short or long
rest, matching the existing class feature's recovery policy. Recovery restores
the pool to its configured capacity; R5 selects one for the first fixture. The restoration
trigger is item/pool configuration, not a special case embedded in Second Wind's
healing behavior. Which character's rest reaches the item depends on the grant's
association/eligibility rules, still to be settled below.

The operator's ultimate goal is to author this configuration in the studio.
Exercise explicit authored configuration in the example; do not hard-code the
policy by ability ref or build the studio editor in this slice. This selects an
authoring direction, not a schema, UI or exhaustive list of recharge mechanisms.
It does not select a recharge rule for the illustrative Raging ring or hotdog.

**R5 — One charge is enough for the first consumer fixture.** Settled by
KirkDiggler, 2026-11-04. Use capacity one, then focus the investigation on an
example package consuming the effects tools rather than serial final-configuration
checks. Capacity remains authored configuration; this does not fix every item's
budget to one. Explicit holder association in a probe is test setup, not a
production equipment/attunement ruling.

## Experimental evidence (non-normative)

### Consumer probe checkpoint

The first cut of [PR #2273](https://github.com/KirkDiggler/rpg-toolkit/pull/2273)
was test-only wiring. After the operator requested continuing the item package,
it became an exported **experimental** composer under `examples/items`, consuming
the merged effects cleanup, existing numerical resources and a separate test
consumer that calls real healing arithmetic.

The candidate separates these responsibilities:

| Surface | Owns / refuses |
|---|---|
| `Definition` | Authored grant refs/parameters and named pool policy; no runtime hooks |
| `New(Config)` / `Item` | Validated instance identity and pool mutation; no rulebook imports |
| `Attach` / `Detach` | Real effects grant/recovery outputs on the caller's bus; no equipment eligibility decision |
| `Offers` | Copied declaration/source/budget views; not complete actor affordability |
| `Use[Result]` | Item charge orchestration and a typed consumer outcome; no world delivery or rule interpretation |
| `Data` | Identity, definition ref and remaining pool counts; no bus, closure or projected offer |

This is evidence for a boundary, not acceptance of its spelling or full schema:

- Item-instance identity is explicit, rather than inferred from recipe-local
  output IDs. Two items remain independent; explicit pool references can express
  sharing within one item. Returned views cannot mutate the live instance.
- The consumer validates/interprets its ability parameters. The prototype uses
  `example:abilities:second_wind`, not a claim that the production feature loader
  accepts this new grant schema. Healing arithmetic is reused; full class-feature
  definition/activation reuse is not yet demonstrated.
- Only the use operation is generic, so its typed result can cross the boundary
  without a D&D import or an untyped/JSON result conversion in Item. Its resolver
  must compute a value without world delivery or reentrant item mutation; refund
  is local to that contract, not arbitrary transactional rollback.
- The grant does not yet cross production `actions.Definition`, actor action-economy,
  resolution-machine or HP-delivery boundaries. Do not charge its item pool in two places.
- The generated effects recipe has one attached state and no mutable bag. Pool
  state belongs to Item; a host must snapshot it after use/recovery. Arbitrary
  passive/stateful item recipes, interruption flows and automatic state-reporting
  protocols are not proved by this cut.

The existing top-level [`items` module](../../items/item.go) declares property and
equipment interfaces, not this grant/pool composition. The example neither
replaces those interfaces nor fixes its extraction home by name.

The next focus is inspecting this **candidate grant/activation contract** and
exercising a contrasting consumer where needed, not more final item settings.
The [journal checkpoint](https://github.com/KirkDiggler/rpg-project/blob/docs/second-wind-ring/docs/journal/second-wind-ring/2026-11-05-from-probe-to-item-composer.md)
records the commands, mutations, CI correction and remaining limits.

## Open integration and extraction questions

These questions guide subsequent experiments and implementation design. They are
not silently answered by acceptance of the ownership decision, nor a requirement
to finish the whole game before recording that decision.

1. **Pool sharing in authored content:** the candidate can bind grants to the
   same named item pool and tests that ownership. Which real abilities/items
   should share one remains a content decision, not a prerequisite to this probe.
2. **Grant lifetime:** when does possession/equipment make the ability available;
   what happens to an already-produced ongoing effect when equipment is removed?
   R2 settles that moving or re-equipping the ring does not refresh its budget.
   Immediate HP already regained is distinct from future access.
3. **Production activation contract:** how do grants map to canonical action
   declarations without teaching resolution content identities, duplicating rules
   or paying twice? The exact `Use[Result]` protocol is not selected by this ADR.
4. **Production identity and reload integration:** how do instance identity and
   irreducible item state connect to existing inventory/definition storage without
   minting uses or duplicating behavior? Their ownership is settled; the storage
   and wire contracts are not.
5. **Content policy, if needed later:** whether and how rarity selects the authored
   level; no rarity table is needed to exercise explicit level-1/level-3 examples.

The operator's pending choices live as a checklist on the PR/slice. Settle only
what is needed for the bounded example; do not turn the ring into a prerequisite
for designing all equipment, spells or conditions.

### Evidence before extraction or production adoption

The isolated example supplies evidence for the accepted ownership boundaries.
The checks below retain the broader integration bar: some are exercised in the
example, while unrun production boundaries remain unproved. They are not being
marked passed or waived by changing this ADR's status.

- A runnable consumer example uses actual reusable components; substitutes and
  unexercised boundaries are identified.
- With a controlled d10 result of 6, level-1 and level-3 ring grants produce
  healing calculations of 7 and 9 for a wearer with no fighter levels. Class-level
  data remains unchanged; actual restored HP still respects the recipient's rules.
- Spending a ring use leaves class-granted and other item-instance pools unchanged.
  Transfer, re-equipping and save/load preserve the remaining ring budget; source
  identity is not confused with the shared ability ref.
- Once rest eligibility is agreed, an eligible short rest and an eligible long
  rest each restore the ring pool to its configured capacity. An unrelated
  character's rest must not refill it merely because a rest event occurred.
- The agreed grant/use/removal and round-trip cases show one owner for each
  mutation and lifetime. Availability agrees with execution; refused uses do
  not silently spend resources.
- Coexisting grants and restoration cases chosen for the use case have named
  assertions, not a claim inferred from one successful activation.
- The record identifies which effects capabilities were consumed, which were
  missing and which were not needed. No successful ring is presented as proof
  of all spell/condition composition.
- Extraction and production plans resolve their affected contracts and receive
  the owning review, operator decisions and required verification. This ADR's
  acceptance does not approve those plans, a merge, or a release.

### Outside this decision

Shared-machine extraction is a different, downstream question. The mind/machine
investigation has its own consumer need; its findings can inform later comparison
without becoming this example's prerequisite. This ADR does not select
`core/machine`, promote `examples/effects`, replace #703's provider slices, migrate
production class features, add studio authoring or deliver the keep's item/loan
journey.

## Consequences

### Expected positive consequences of B

- Item acquisition can introduce new opportunities without a new copy of their
  underlying behavior.
- Grant, cost, instantaneous result and ongoing lifetime can be reasoned about
  and tested separately while remaining one usable composition.
- The ADR can be revised from observed consumer needs rather than an extraction
  chosen because two packages share a name.

### Expected negative consequences of B

- The example must confront ownership and restoration instead of hiding them
  inside a ring-specific method.
- Existing executable features and declarative actions may expose a migration
  gap; the example cannot honestly claim production adoption resolves itself.
- A Second Wind ring may not exercise the ongoing-effect composer enough to
  decide its broader suitability. Additional consumer evidence could be needed.

### Neutral consequences

- The existing Fighter feature remains unchanged during investigation.
- Actual item rules may differ from class rules while sharing a lower operation.
- Revisit or supersede this decision if later consumer evidence shows a different
  ownership boundary is needed. Preserve the reasoning rather than rewriting the
  earlier choice as inevitable; an accepted decision is not a frozen example API.
