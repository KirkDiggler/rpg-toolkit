# ADR-0050: Composed condition instances own bindings, origin and runtime state

Date: 2026-11-07

## Status

Accepted by KirkDiggler through approval of the complete [Raging-only design](../ideas/rulebook-effect-composition/design.md) in [#2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

Acceptance covers the ownership and contracts below, not a claim of implementation, review, release or native integration. [The plan](../ideas/rulebook-effect-composition/plan.md) carries execution; the implementation record reports observed outcomes.

## Context

ADR-0048 separates item-owned grants/pools from reusable rulebook behavior. A shared Raging definition needs to support both a class-derived amount and a fixed item-authored amount without inventing wearer class levels or making each source another Rage implementation.

An active effect can outlive item attachment. Its reconstruction therefore cannot depend on a live item, another encounter or a closure that is absent from storage. At the same time, copying a Barbarian's level or computed bonus into the saved condition would make a derived value authoritative.

The generic machine owns mutable lifecycle state; the rulebook owns named composition, parameter meaning, provenance, persistence and player-facing explanation. The current class-only Rage blob does not express that complete instance contract. The operator explicitly permits recreating affected characters, so historical Rage-format compatibility is not required.

Alternatives considered:

- Preserve or normalize legacy Rage data: rejected because the operator permits recreating affected characters; a second format path would add no required capability.
- Keep a separate item recipe/creator beside native Rage: rejected because it does not converge the actual callers and duplicates behavioral ownership.
- Put rulebook origin, display and projection fields into the generic composer: rejected because these meanings belong to the rulebook protocol adapter; neutral snapshots and output observations are sufficient.

## Decision

1. The existing condition descriptor registry is the sole named-definition catalog. Native Barbarian Rage and the item consumer request the same definition, with fresh independent runtime state.
2. A rulebook composed instance stores validated immutable bindings and origin separately from one mutable runtime snapshot. Bindings select a fixed authored amount or a rulebook-owned class-level strategy; the latter reads current frame data and stores no derived class bonus.
3. Origin identifies granting content and its source entity. Recipient identity and stacking address remain separate. Item provenance does not change Raging's by-ref identity.
4. Creation and restoration use the same definition builder. Restoration requires the complete current representation and refuses obsolete, missing or malformed state rather than creating a fresh instance. No legacy Rage reader, writer, normalization pass or automatic character reset is introduced.
5. The rulebook protocol adapter owns its codec and information integration and delegates execution/lifecycle to mechanics/effects. Typed snapshots and observational output access avoid serializing runtime JSON only to parse it into another shape.
6. One rulebook kind table supplies full-output validation, execution and domain-honest information. Generic tools contain no D&D meanings or arbitrary domain projection payloads. Lifecycle change notification reaches native persistence; terminal removal remains publish-before-detach.
7. Activation owns access, payment and delivery. Item/class pools and active effect state do not share a mutation owner. API/web consume toolkit-owned facts and do not reconstruct bonus rules.

This cut migrates Raging only. Other conditions retain their current constructors and formats until their own deliberate migration. The exact current data and proposed/implemented symbols are described in the [walkthrough](../ideas/rulebook-effect-composition/README.md); this ADR does not duplicate that field specification.

## Consequences

### Positive

- Item and class grants reuse one behavioral definition without sharing mutable state or budgets.
- Saved instances retain the facts needed for reconstruction after source detachment, while class-derived amounts remain live.
- Creation, load, execution and information have explicit refusal boundaries and one owner each.

### Negative

- Characters holding incompatible old Rage data must be recreated explicitly.
- The rulebook must implement a codec/protocol adapter, and the generic provider needs the neutral capabilities the consumer exercises.
- Native fixture/pin and reader adoption must be verified across the real stack, not inferred from example tests.

### Neutral

- Go-authored definitions and closed vocabulary remain the authoring mechanism; no studio or expression language is introduced.
- Shared-machine extraction, catalog-wide migration, feature-to-actions conversion and native inventory are separate work.
- Budget delivery failures retain their documented boundaries; this is not distributed rollback.
