# ADR-0031: Encounter event spine — causation, game-event time, and the resolved-action event

Date: 2026-06-01

## Status

Accepted

## Context

The encounter event spine (`encounter/events`) is the canonical record of what
happened in a fight — it is what a toolkit-owned combat log (North-Star
Invariant 3) will be reassembled from, and what rpg-api projects to the wire.
Before this change the spine carried only `EncounterID()`, `Sequence()`, and
`Audience()` per event. Three gaps blocked a faithful, reassemblable story (the
TakeAction wave, rpg-project#54 / rpg-toolkit#697):

1. **No causation link.** The only thing tying an `AttackResolvedEvent` to its
   `DamageDealtEvent` was adjacent `nextSeq()` values — fragile and implicit
   (Invariant 8). Insert any event between them, or reorder, and the link
   breaks.
2. **No game-event time.** The spine carried only a sequence number; rpg-api
   stamped `now` at translate-time, which is wire-delivery order, not game order
   (Invariant 5).
3. **No first-class "an action was taken" event.** Only attack-shaped
   `AttackResolvedEvent` existed. Non-attack actions (Dodge, Dash, the Monk
   bonus strike) had no canonical beat, and even an attack scattered its meaning
   across roll-detail + damage events with no umbrella record carrying the
   action ref or what it cost the economy (Invariant 9).

## Decision

Add three things to the spine, shaped now to be forward-compatible with a
durable combat log (persistence itself stays deferred, Invariant 13):

- **`OccurredAt() time.Time` + `CorrelationID() core.CorrelationID` +
  `Stamp(at, corr)` on the `EncounterEvent` interface.** All three are
  single-sourced on an embedded `eventMeta` struct, so every concrete event
  satisfies them for free and adding a future spine field touches one struct,
  not all ~17 events. The two fields serialize inline via an embedded `metaWire`
  (`occurred_at`, `correlation_id`).

- **The `Broker` is the single timestamp authority.** `Broker.Publish` stamps
  game-event time from an injected `core.Clock` at the literal publish moment,
  preserving any correlation id the encounter already set. Production uses
  `SystemClock`; tests inject `FixedClock` via `NewBrokerWithClock` and assert
  `OccurredAt` exactly. "Game-event time at publish" is therefore literal, with
  no per-call-site boilerplate across the ~23 publish sites.

- **`ActionResolvedEvent` — a first-class resolved-action event.** Carries
  `ActorID`, `ActionRef` (string, so the SDK stays free of rulebook ref types),
  optional `TargetID`, and `EconomyConsumed` (actions / bonus actions /
  reactions / movement + a `granted_consumed` map). It is the cause beat every
  action emits; attack roll detail stays on the parallel `AttackResolvedEvent`.
  The encounter sets one correlation id — derived from this event's own
  `(encID, sequence)` identity — on the resolved-action event and every effect
  event of the same resolution.

The correlation id is derived from the existing monotonic sequence
(`corr-<encID>-<seq>`) rather than a UUID: deterministic, dependency-free, and
trivially assertable in tests, while still unique per action within an
encounter.

## Consequences

### Positive
- A consumer reassembles "this damage came from that action" from the
  correlation id, not from sequence adjacency — the combat log is buildable
  later (Inv 3) with no retrofitting (Inv 8).
- Event order reflects game time, not delivery time (Inv 5), deterministically
  testable via the injected clock.
- Every action — attack and non-attack — has one canonical event carrying its
  ref and economy cost (Inv 9), which the menu/economy unification PR populates
  with richer consumption.
- Adding spine fields later is a one-struct change (`eventMeta`).

### Negative
- Every event now carries two extra fields on the wire (`occurred_at`,
  `correlation_id`); `correlation_id` is `omitempty` for events not part of a
  correlated action group (mode changes, turn boundaries).
- The proto envelope must mirror the two new fields + the new `ActionResolved`
  oneof variant before rpg-api stops suppressing — a proto gap to close
  (Inv 7), tracked under the wave, not a permitted drop.

### Neutral
- `EconomyConsumed` on the attack path reports the honest known cost today
  (`Actions: 1`); the menu/economy unification PR sources the richer two-level
  consumption from the character action economy.
- The attack publish path threads a single `attackActionRef` constant for now;
  the unification PR generalizes `applyAndPublishOutcome` to carry the actor's
  submitted ref so non-attack actions report their own.

## Example

```go
// Broker stamps game-event time at the literal publish moment.
broker := encounter.NewBrokerWithClock(transport, core.FixedClock{At: gameTime})

// During one TakeAction resolution the encounter publishes a correlated group:
//   ActionResolvedEvent{ActionRef:"dnd5e:action:attack", EconomyConsumed:{Actions:1}}
//   AttackResolvedEvent{Hit:true, AttackRoll:17, ...}
//   DamageDealtEvent{Amount:8, ...}
// all three sharing corr-<encID>-<actionSeq> and OccurredAt == gameTime.
```
