# ADR-0046: Encounter Owns Location Knowledge

**Date:** 2026-08-28
**Status:** Accepted

## Context

`play/intel` records channel-sourced testimony as opaque payloads. That is a
deliberate leaf contract: Intel can say who reported what, through which
channel, and whether that testimony is current or held, but it cannot interpret
geometry or compare testimony with concealed world truth.

The encounter composition already owns the missing context. It constructs
lawful sight percepts, knows the field geometry, projects fight-time driver
views, and observes when a driven member reaches a cell. A monster that loses
sight of a player therefore needs encounter-owned vocabulary for two different
beliefs: a remembered coordinate it can investigate, and continued knowledge
of the subject with no actionable coordinate.

Treating a nil payload, zero coordinate, faded channel, or deleted holding as
"unknown" would collapse different facts. It would also either teach Intel
geometry or force behavior and session to reinterpret opaque payloads.

## Considered alternatives

### Put location semantics in `play/intel`

Rejected. Intel is a channel-oriented storage leaf whose payload is deliberately
opaque. Giving it known/unknown location types, geometry, or truth comparison
would make the generic testimony store depend on encounter rules and would
blur which layer may observe concealed world state.

### Treat nil, fade, or deletion as unknown location

Rejected. A nil payload is malformed testimony, fading changes testimony from
current to held, and deleting a holding forgets the subject entirely. None of
those states expresses durable knowledge of a subject without an actionable
position. Overloading them would also make a valid zero coordinate ambiguous.

### Decode and correct location testimony in behavior or session

Rejected. Behavior receives a projected monster view and has neither encounter
geometry nor a complete lawful percept. Session is a load-act-save host seam;
having it decode or correct payloads would duplicate rule semantics and risk
consulting concealed state outside the encounter boundary.

### Let encounter own location testimony and correction

Selected. Encounter authors sight testimony, owns field geometry, produces the
complete lawful percept after movement, and propagates the resulting Intel
deltas. Behavior can remain a view consumer, session can remain a mirror, and
Intel can remain an opaque store.

## Decision

### Location testimony belongs to encounter

Encounter owns one strict typed value:

```text
Known(position) | Unknown
```

The canonical persisted payload is tagged. Known testimony is
`{"state":"known","x":X,"y":Y}`; unknown testimony is
`{"state":"unknown"}`. `play/intel` continues to store either payload without
interpreting it and gains no position API, geometry, or world truth.

Intel currency and location content remain independent:

- `Current + Known` projects into `MonsterView.Seen`;
- `Held + Known` projects separately into `MonsterView.Remembered`;
- `Held + Unknown` remains persisted knowledge but projects into neither
  actionable collection; and
- `Current + Unknown` is rejected because this rung gives it no lawful meaning.

A remembered entry is testimony, not a live target. It contains no concealed
standing or reach facts, is never attackable, and carries a shortest path that
ends on the exact remembered cell. Current sight keeps its existing reach-aware
path and attack facts in `Seen`.

The reference `behavior.Basic` driver evaluates a fresh view on every driver
call. Its visible branch owns the decision whenever a visible standing player
exists, even when the available path or budget makes that branch return
`Pass`. It considers the closest reachable remembered player only when no
visible standing player exists. A player newly visible after a
remembered-directed step therefore interrupts remembered pursuit on the next
call. This decision adds no sticky target commitment.

### Encounter authors arrival correction

After a successful fight-time `TurnDriver` move, encounter refreshes sight and
may replace stale known testimony at the mover's exact arrival cell with
unknown testimony. The correction is based only on:

- the mover's own prior held Intel;
- the mover's arrival cell; and
- the complete lawful percept produced by that movement refresh.

It does not read or reveal the absent subject's concealed live position. The
correction preserves the holding and subject, reports through the existing
Sight channel, and remains `Held` because no channel currently sustains it.

Correction is encounter-authored testimony and must not be a silent mutation.
Encounter-owned `IntelDelta` values carry `Corrected` subjects alongside the
projected `FirstContact`, `Refreshed`, and `Faded` transitions. Every enclosing
encounter output that can drive a turn propagates those deltas. Public `Step`
and free-roam `Pump` do not independently perform arrival correction.

### Session mirrors; it does not decide

Session mirrors explicit known/unknown state on sight-channel holdings,
projects remembered view data by value, and returns correction observer/subject
identifiers. It neither decodes location JSON itself nor decides whether a
memory is actionable, which subject a driver selects, or when correction is
lawful. Its load-act-save boundary persists the encounter result only after the
whole verb succeeds.

## Compatibility and validation

Existing untagged `{"x":X,"y":Y}` sight payloads remain readable as known
locations. New writes always use the tagged canonical form. The compatibility
`DecodeSightPayload` surface continues to return only known positions, while
state-aware callers use `DecodeLocationPayload`.

Encounter rejects malformed JSON, unsupported or duplicate fields, unsupported
states, missing coordinates, unknown testimony carrying coordinates, and
trailing data. It also rejects current unknown sight testimony. Payloads from
other Intel channels remain opaque and are not interpreted merely because they
contain coordinate-like data.

## Consequences

### Positive

- A monster can investigate stale belief and correct it without receiving
  concealed world truth.
- Current sight keeps its existing meaning; held memory is additive and
  visibly non-attackable.
- Zero and negative coordinates remain valid known positions, while unknown
  is explicit and durable across persistence.
- Corrections are observable, persistable deltas rather than hidden mutations.
- `play/intel`, behavior, and session retain their existing layer boundaries.

### Negative

- Encounter must strictly validate its sight payload dialect during load, so
  fixtures and providers that persisted nil or malformed sight payloads must
  provide lawful testimony.
- Encounter and session outputs that can enclose a driven turn carry an
  additive correction projection.
- A shortest path is computed for every actionable remembered entry on each
  driver view, even though the driver may choose only one.

### Deferred

This first rung does not add route-utility scoring, correction when a route
merely observes a remembered cell, persisted target commitment, freshness or
channel-reliability policy, richer multi-target comparison, search after an
unknown location, hiding, invisibility, sound physics, or deception behavior.

## Rule

**A composition interprets and corrects the testimony whose meaning depends on
its geometry and lawful percept; the storage leaf remains opaque, and the host
seam mirrors the result without deciding it.**
