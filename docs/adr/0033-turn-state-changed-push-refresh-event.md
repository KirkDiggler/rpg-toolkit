# ADR-0033: TurnStateChangedEvent — push-refresh the menu/economy

Date: 2026-06-01

## Status

Accepted

## Context

North-Star Invariant 12: *"economy/menu refresh is pushed as a delta event — the
menu never goes silently stale."* After the TakeAction unification (ADR-0032)
the toolkit computes the per-actor action menu + economy and exposes it as a read
surface (`Encounter.ActorTurnState`), but nothing **pushed** it: a client would
have to poll `ActorTurnState` to learn its menu changed after taking an action.

The encounter broker only carries toolkit `EncounterEvent` types. So rpg-api
could not push a `TurnStateChanged` to the web on its own — synthesizing one in
the API would author rules-state and bypass the spine (Invariant 2). The
push-refresh is therefore a toolkit gap: the engine must emit the delta event.

## Decision

Add a `TurnStateChangedEvent` to the encounter spine (`encounter/events`) and
publish it whenever an actor's turn state mutates.

- **Rulebook-agnostic snapshot.** The spine must not import rulebook types
  (`encounter/events` carries none today). The event carries a flattened
  `TurnStateSnapshot` — economy counters (ints) + a `[]MenuEntry` of
  ref/name/economy-slot/target-kind/available/reason primitives — exactly the
  `EconomyConsumed`-style flattening already used on the spine. The encounter
  builds the snapshot from its rulebook-aware `ActorTurnState` at publish time
  (`buildTurnStateSnapshot`).
- **Emitted on every turn-state mutation.** Turn start (`seedActorTurn`, after
  `character.StartTurn`) and every action taken — the non-attack path
  (`takeCharacterAction`, after the economy diff) and the attack path
  (`applyAndPublishOutcome`, after `publishAttackOutcome`). One helper,
  `Encounter.publishTurnStateChanged`, owns the build + publish.
- **Audience is the actor's controlling player.** Turn state is that player's
  private "what can I do now" view, so the push is audience-of-one. A no-op for
  NPCs (no controlling seat) and flat-stat seats (no character menu/economy).
- **Correlated to its cause (Invariant 8).** The post-action push carries the
  same correlation id as the `ActionResolvedEvent` that triggered it (the
  attack/non-attack publish helpers now return the corr id they stamped). The
  turn-start push carries no correlation id — it is not caused by an action.
- **Stamped with game-event time (Invariant 5)** via the embedded `eventMeta`,
  like every spine event; registered in the broker codec.

rpg-api translates this to the proto `TurnStateChanged` (envelope field 45,
already on the wire), field-for-field.

## Consequences

### Positive
- The client menu/economy updates live off the stream; it never goes silently
  stale (Inv 12). No polling.
- The push is a full snapshot, not a diff — the client always has the complete
  current menu and cannot drift from missed deltas.
- The spine stays rulebook-agnostic (primitives only); any rulebook host can
  consume the event.
- The post-action push is reassemblable into the combat log with its action via
  the shared correlation id (Inv 8).

### Negative
- A full-snapshot push is larger on the wire than a minimal delta. Acceptable:
  the menu is small and correctness/simplicity (no drift) outweigh the bytes.
- One more event type the broker codec + any consumer must handle.

### Neutral
- Movement-step economy deltas (Beat 2) will reuse the same event from the
  movement path when that lands — no new event shape needed.

## Example

```go
// After any turn-state mutation the encounter pushes the fresh snapshot:
//   seedActorTurn  -> publishTurnStateChanged(actor, "")        // turn start
//   takeCharacterAction -> publishTurnStateChanged(actor, corr) // non-attack
//   applyAndPublishOutcome -> publishTurnStateChanged(actor, corr) // attack
//
// TurnStateChangedEvent{ ActorID, State:{InCombat, economy counters, Menu:[...]} }
// audience = the actor's controlling player; OccurredAt stamped at publish;
// CorrelationID = the causing action's id (empty for turn start).
```

## Related

- ADR-0031 — the event spine (causation id + game-event time + embedded
  `eventMeta`) this event is built on.
- ADR-0032 — the TakeAction unification that produced `ActorTurnState`, the read
  surface this event snapshots.
