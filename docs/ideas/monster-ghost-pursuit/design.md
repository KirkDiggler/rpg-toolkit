# Monster ghost pursuit — design

**Status:** Approved

**Execution:** [plan.md](plan.md)

**Product direction:** [rpg-project#201](https://github.com/KirkDiggler/rpg-project/issues/201)

**Reconciliation:** [rpg-project#305](https://github.com/KirkDiggler/rpg-project/issues/305) and [rpg-project PR #306](https://github.com/KirkDiggler/rpg-project/pull/306) are unmerged design input, not toolkit authority.

**Rules authority:** System Reference Document 5.1. Roll20's 2024 Free Basic Rules may clarify terminology but do not override SRD 5.1.

## Outcome

A fight-time monster with no visible player may pursue the closest player position it remembers. It walks to that exact cell using only its own Intel. If it reaches the cell and does not perceive that player there, the player remains a known subject but their position becomes explicitly unknown. The resolved location is no longer actionable, so the monster does not repeatedly pursue it.

This is the first behavior rung for issue 201. It proves that a monster can act intentionally on stale belief and correct that belief without receiving another member's concealed live position. It is not a general monster-planning system.

## Existing contracts

The change preserves these boundaries:

- `play/intel` stores opaque, channel-sourced testimony that may be false or stale. It does not inspect payloads, geometry, or world truth.
- `rulebooks/dnd5e/encounter` is the composition. It constructs lawful percepts, owns geometry and line of sight, authors location testimony, projects driver views, and detects arrival at a remembered cell.
- `rulebooks/dnd5e/behavior` owns target-selection policy and consumes plain view data. It never queries a live encounter.
- `rulebooks/dnd5e/session` remains a load-act-save host seam. It projects and persists results but owns no rule or behavior decision.
- SRD 5.1 governs D&D mechanics. Persistent belief, stale positions, and target policy are project behavior rather than SRD rules.

`intel.Surveil` is not changed. Its complete-percept fade removes a sustaining channel while preserving the last payload, which is exactly how a current sighting becomes a held ghost.

## Laws

- **G1 — Intel remains opaque.** Positional meaning is encoded and interpreted by the encounter composition. `play/intel` gains no geometry, truth access, or position-specific state.
- **G2 — Currency and content are independent.** Intel's derived `Current`/`Held` status says whether testimony is actively sustained. Encounter-owned location testimony separately says `Known(position)` or `Unknown`.
- **G3 — Views contain knowledge, never concealed truth.** A monster receives its own facts, current sightings, and its own actionable remembered positions. It never receives a subject's hidden current position.
- **G4 — Live sight keeps its meaning.** `MonsterView.Seen` remains current sight Intel. Remembered positions are exposed separately and never folded into `Seen`.
- **G5 — Ghosts are not attack targets.** Remembered entries carry no attack-reach facts. Their paths end on the exact remembered cell rather than at the nearest cell within attack range.
- **G6 — Visible knowledge interrupts remembered pursuit.** `behavior.Basic` selects a visible standing player whenever one is available. It considers remembered positions only when no visible player is actionable.
- **G7 — First-rung remembered selection is deterministic.** With no visible player, `Basic` selects the closest actionable remembered player. Equal distances temporarily break by subject ID.
- **G8 — First-rung correction occurs at arrival.** Reaching the exact remembered cell and not perceiving that subject there replaces the known coordinate with explicit unknown location testimony. Merely bringing the cell into sight does not resolve it in this rung.
- **G9 — Correction preserves the subject.** Unknown position removes the subject from actionable movement candidates; it does not delete the holding or claim the subject ceased to exist.
- **G10 — New sight replaces uncertainty.** A later current sighting overwrites unknown or stale positional testimony through the existing latest-accepted-testimony rule.

## Location testimony

The encounter owns one strict tagged location-testimony value with two states:

```text
Known(position)
Unknown
```

The serialized shape must distinguish these states explicitly. Zero and negative coordinates, nil or empty payloads, and malformed data are not unknown sentinels. Existing persisted coordinate payloads remain readable as known positions.

Valid combinations for this rung are:

| Intel status | Location state | Meaning | Actionable |
|---|---|---|---|
| Current | Known | The observer currently perceives the subject at this position. | Yes, as `Seen` |
| Held | Known | The observer remembers or was told this position. | Yes, as `Remembered` |
| Held | Unknown | The observer knows the subject but has no actionable position. | No |

`Current + Unknown` has no meaning in this rung and is rejected by encounter-owned validation rather than given an invented interpretation.

The testimony is channel-neutral even though today's coordinate type is named `SightPayload`. Sight, sound, deception, or another encounter-authored channel may testify to a location. Intel records the channel without judging the testimony's truth.

## Fight-time view

`MonsterView` gains a separate remembered collection. Each actionable remembered entry contains only:

- subject/member ID;
- member kind;
- remembered position;
- distance from the observing monster;
- shortest path ending on the remembered cell.

It does not contain standing state derived from concealed world truth, per-action `InReach`, or the subject's current position. Held unknown testimony remains persisted Intel but produces no remembered entry.

Timestamp and channel provenance remain available in Intel but are not projected into the first-rung remembered view. They may be added when an authored policy actually uses freshness or source reliability.

A subject cannot appear simultaneously in `Seen` and `Remembered`. Current positional testimony supersedes its stale candidate.

## Basic behavior

For each driver call, `behavior.Basic` applies this order:

1. Select the closest standing player in `Seen`, retaining the existing deterministic ID tie-break.
2. Attack that visible player when an authored action is in reach and attack budget remains.
3. Otherwise move one cell along that visible player's path when movement remains.
4. Only when no visible player is actionable, select the closest player in `Remembered`, with a temporary subject-ID tie-break.
5. Move one cell along the remembered path when movement remains.
6. Otherwise pass.

Because `Basic` emits one-cell moves and the encounter rebuilds the view after movement, a newly visible player interrupts remembered pursuit on the next driver call. No persistent target commitment is introduced in this rung.

## Arrival correction

After movement and the existing sight refresh, the encounter performs arrival correction for the moving observer:

1. Read only that observer's held location testimony.
2. Find known remembered positions equal to the observer's destination cell.
3. Compare their subjects with the observer's complete lawful percept.
4. For each absent subject, submit discrete corrective testimony whose location state is `Unknown`.
5. Surface the correction in encounter-owned Intel deltas and persistence; do not mutate belief silently.

The existing `intel.Report` verb can store this correction: it replaces the opaque payload while leaving `CurrentVia` empty, so the holding remains `Held`. The implementation design must give this observation-derived report an explicit encounter contract and project its returned correction honestly.

The correction does not inspect or reveal the subject's hidden current position. World state may be used to construct the lawful percept; behavior and belief correction use only that percept, the observer's prior Intel, and the observer's arrival cell.

## Failure and compatibility

The encounter rejects unsupported location tags, known states without positions, unknown states carrying positions, malformed payloads, and trailing data. Existing valid coordinate payloads continue to decode as known positions.

Encounter operations retain their load-act-save failure contract. If correction, projection, or persistence fails, the operation returns an error and the caller discards the unsaved encounter. Movement must not be reported as durably successful while a required belief correction is silently lost.

Unknown position is distinct from malformed testimony in session projections and returned deltas. Reloading corrected Intel must not resurrect the former coordinate.

## Acceptance criteria

### Location testimony

- Existing coordinate payloads decode as known positions.
- Known and unknown states round-trip.
- Cell `(0,0)` remains a valid known position.
- Malformed and contradictory states are rejected.

### View projection

- Current known testimony becomes `Seen`.
- Held known testimony becomes `Remembered`.
- Held unknown testimony becomes neither actionable collection.
- A subject never appears in both collections.
- A remembered path ends on the exact remembered cell and carries no attack-reach data.
- No concealed current position reaches the driver.

### Basic behavior

- Multiple visible players retain closest-visible selection.
- A visible player takes priority over every remembered player.
- With no visible player, the closest remembered player is selected.
- Equal remembered distances break by subject ID.
- A remembered player is never attacked.
- No actionable knowledge returns `Pass`.

### Encounter proof

The discriminating double-door fixture proves:

1. The monster sees Billy at the carpet cell.
2. Billy moves through the intervening space and behind the second door, breaking sight.
3. The monster retains a held known position at the carpet cell and receives no hidden live position.
4. No other player is visible, so the monster moves to the exact remembered cell.
5. On arrival, Billy is absent from the monster's percept; the encounter changes Billy's location testimony to unknown.
6. On the next decision, the monster does not pursue the carpet cell again.

The proof does not require the monster to find Billy, choose a later door, or complete in a fixed number of turns.

An interruption test separately proves that a player becoming visible during remembered pursuit takes priority on the next driver call.

### Persistence and seam

- Known and unknown location testimony survive encounter and session round-trips.
- Session distinguishes unknown position from malformed testimony.
- Reload after correction does not restore the stale coordinate.

## Deferred behavior

The design preserves these future requirements without implementing them:

- Prefer a route toward a visible target when that route also checks a remembered location.
- Otherwise compare direct investigation cost with the closest visible-target distance: investigate the memory on an equal cost and prefer the visible target when the memory is farther.
- Correct remembered Intel when a route lawfully observes its location, rather than only on exact-cell arrival.
- Separate belief correction from continued investigation when a behavior remains committed after learning the old coordinate is empty.
- Use freshness, channel reliability, disposition, or other authored facts in selection.
- Replace temporary ID tie-breaks when richer seen/remembered policy makes a better deterministic choice available.
- Add persisted per-monster commitment only when a multi-target test demonstrates oscillation that stateless selection cannot represent.

This rung does not implement hiding, invisibility, sound physics, deception behavior, search after unknown position, route-utility scoring, or a general planning system.

## Source-of-truth obligations

Implementation requires focused updates to the encounter location-payload and turn-driver godoc, session projection documentation, persistence contracts, and their tests. The existing meaning of `Seen` must be stated as unchanged.

The durable decision that location knowledge is encounter-owned and correction is composition-authored should receive an ADR. ADR-0043's stale `Proposed` status should be reconciled separately if current repository authority confirms it is implemented; that cleanup is not license to rewrite unrelated historical encounter documents.

After implementation, this idea gains `implementation.md` recording shipped paths, tags, verification, and deviations before its PR merges. Project issue 201 and the open issue 305 / PR 306 should then be reconciled against observed toolkit behavior.
