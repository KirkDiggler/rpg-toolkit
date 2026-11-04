# ADR-0048: Composing Equipment-Granted Abilities and Effects

Date: 2026-11-04

## Status

Proposed — architectural investigation in progress. The ring-authored ability
level is settled below; composition/API, uses and grant lifetime remain open.

Consumer slice: [Keep signature combat and loot S1 — Second Wind ring](https://github.com/KirkDiggler/rpg-project/issues/862),
under [#850](https://github.com/KirkDiggler/rpg-project/issues/850).
Existing effects work remains owned by [#703](https://github.com/KirkDiggler/rpg-project/issues/703).

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

## Alternatives under investigation

### A. Grant the existing feature object directly

Reuse `features.SecondWind` with a new source of ownership. This is the shortest
route to exercising the current class ability, its uses and rest behavior.
However, it carries fighter eligibility, fighter-level scaling, private resource
ownership and executable-feature activation. This option was considered while
fighter-only ring behavior was still open. The operator's item-authored level
requirement below means unmodified reuse cannot supply the ring's intended level;
keep this option as the baseline, not a reason to alter the wearer's class levels.

### B. Compose the grant with reusable activation and effect behavior

**Recommended direction to investigate, not an accepted API.** The item grants
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

No implementation choice is accepted yet. Investigate B against the ring use case,
using A's existing behavior as a comparison baseline. The operator has authorized
the ADR/investigation and the item-level parameter below, not a public API,
migration or the remaining item rules.

The proposed separation to exercise is:

```text
ring / other source
        → grant makes an ability available
        → activation validates and pays its declared cost
        → immediate consequence and/or ongoing effect
```

These are responsibilities to examine, not four required new packages. Sharing
underlying behavior does not mean that an immediate heal must become a persistent
condition or that every ability needs a state machine.

The example remains under `examples/`; its package name, module boundary,
consumer vocabulary and public interfaces are decided in the checked plan after
the use case and composition are agreed. Reuse the existing effects prototype
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

### Open requirements and decisions

1. **Uses and restoration:** which use pool belongs to the item or beneficiary;
   how does it coexist with a class-granted use; what restores it?
2. **Grant lifetime:** when does possession/equipment make the ability available;
   what survives removal, re-equipping or transfer; what happens to an effect
   already produced? Immediate HP already regained is distinct from future access.
3. **Consumer contract:** what form makes the granted ability inspectable and
   executable without content-ref dispatch, duplicate rules or double payment?
4. **Identity and reload:** what distinguishes separate grants and which mutable
   facts must round-trip without minting uses or duplicating behavior?
5. **Content policy, if needed later:** whether and how rarity selects the authored
   level; no rarity table is needed to exercise explicit level-1/level-3 examples.

The operator's pending choices live as a checklist on the PR/slice. Settle only
what is needed for the bounded example; do not turn the ring into a prerequisite
for designing all equipment, spells or conditions.

### Evidence required before accepting the architectural choice

- A runnable consumer example uses actual reusable components; substitutes and
  unexercised boundaries are identified.
- With a controlled d10 result of 6, level-1 and level-3 ring grants produce
  healing calculations of 7 and 9 for a wearer with no fighter levels. Class-level
  data remains unchanged; actual restored HP still respects the recipient's rules.
- The agreed grant/use/removal and round-trip cases show one owner for each
  mutation and lifetime. Availability agrees with execution; refused uses do
  not silently spend resources.
- Coexisting grants and restoration cases chosen for the use case have named
  assertions, not a claim inferred from one successful activation.
- The record identifies which effects capabilities were consumed, which were
  missing and which were not needed. No successful ring is presented as proof
  of all spell/condition composition.
- Evidence and alternatives support the decision, then the owning review and
  operator agreement are recorded before changing status to Accepted.

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
- Revisit the recommendation if the agreed use case is fully served by existing
  contracts, if a proposed shared boundary requires unrelated consumer policy,
  or if the example cannot establish a single owner for state and mutation.
