# Issue 201 update — first monster behavior planned

The first implementation slice for this journey is now shaped against the current composable encounter/session stack.

## The behavior

A fight-time monster that loses sight of a player may act on the position it remembers. When no player is currently visible, it walks to the closest actionable remembered player position. If it reaches that exact cell and does not perceive the player there, the player remains a known subject but their position becomes explicitly unknown. On the next decision, the resolved location is no longer actionable, so the monster does not pursue the same empty cell forever.

This is intentionally one behavior rung, not a general monster-AI system.

## Why this is the discriminating first proof

The proof has to distinguish both missing halves:

- **Decision:** after sight breaks, current `behavior.Basic` passes because it considers `Seen` only. Moving toward the remembered cell proves the driver can act on stale belief.
- **Belief correction:** stopping the test after the first remembered-directed move would miss pillar-camping. Reaching the cell, changing the position to unknown, and not selecting it again proves the world model can resolve stale belief.

The monster is not expected to find the hidden player, select a particular later door, or finish in a fixed number of turns.

## Agreed ownership

- `play/intel` remains a generic, opaque testimony store. It may hold false or stale information, but it does not understand positions, geometry, or truth.
- `rulebooks/dnd5e/encounter` owns location testimony, lawful perception, exact-cell path construction, arrival detection, and corrective testimony.
- `rulebooks/dnd5e/behavior.Basic` owns the first target-selection rung and consumes plain view data only.
- `rulebooks/dnd5e/session` mirrors and persists the result through its existing load-act-save seam; it owns no behavior rule.

The encounter may use world state to construct a monster's lawful percept. It must never expose a concealed player's live position to the behavior driver. Belief correction is derived from the monster's prior Intel, its arrival cell, and its complete percept—not from knowledge of where the hidden player actually went.

SRD 5.1 remains the rulebook authority. The persistent belief and targeting policy here are project behavior layered around those rules.

## Location knowledge

Encounter-owned location testimony gains two explicit states:

```text
Known(position)
Unknown
```

Intel currency and location content remain separate:

| Intel status | Location state | Meaning | Actionable |
|---|---|---|---|
| Current | Known | The monster currently perceives the subject at this position. | Yes, through `Seen` |
| Held | Known | The monster remembers or was told this position. | Yes, through `Remembered` |
| Held | Unknown | The monster knows the subject but has no usable position. | No |

`Surveil` keeps its existing fade behavior: losing sight changes current testimony into a held ghost while retaining the last payload. On exact-cell arrival, the encounter uses discrete corrective testimony to replace the stale coordinate with explicit unknown. No `play/intel` change is planned.

The new tagged payload remains backward compatible with persisted coordinate testimony. Zero and negative coordinates remain valid; nil, empty, malformed, or magic coordinates are not used as unknown sentinels.

## Fight-time view and first selection policy

`MonsterView.Seen` keeps its current meaning. A separate `Remembered` collection carries only:

- subject/member ID;
- member kind;
- remembered position;
- distance from the observing monster; and
- shortest path ending on the exact remembered cell.

Remembered entries carry no hidden standing state and no attack-reach facts. A ghost cannot be attacked.

For this first rung, `Basic` decides in this order:

1. Select the closest visible standing player.
2. Attack that player when legal; otherwise move toward them.
3. Only when no visible standing player exists, select the closest reachable remembered player position.
4. Break equal remembered distances by subject ID for deterministic behavior.
5. Move one cell toward that remembered position.
6. Pass when no actionable knowledge remains.

If a player becomes visible during remembered pursuit, the next driver call returns to visible-target behavior.

## Multiple targets: decided now and deferred

The first slice supports ordinary party encounters rather than assuming one player. Multiple visible players and multiple memories are valid inputs, but the initial policy stays deliberately small.

The future sentient policy is recorded without being implemented yet:

- Prefer a route toward a visible target when that route can also check remembered Intel.
- Otherwise compare direct investigation cost with the closest visible-target distance.
- Investigate the memory on an equal cost; prefer the visible target when the memory is farther.
- Replace the temporary ID tie-break when richer seen/remembered facts support a better deterministic decision.
- Introduce persisted target commitment only after a multi-target test demonstrates oscillation that stateless selection cannot represent.

That future behavior may correct a remembered location when a route brings it into lawful view. This first rung corrects only on exact-cell arrival.

## Implementation sequence

The toolkit plan has eight reviewable tasks:

1. Add strict, backward-compatible encounter-owned location testimony.
2. Replace raw `intel.SurveilOutput` exposure with an encounter-owned Intel delta that can report correction.
3. Project held known positions into `MonsterView.Remembered` with exact-cell paths.
4. Correct remembered location on a driven monster's exact-cell arrival and persist the unknown state.
5. Extend `behavior.Basic` with visible-first, remembered-fallback selection.
6. Carry remembered knowledge and correction deltas through both session adapters and load-act-save persistence.
7. Prove the double-door behavior and visible-target interruption through the real session seam.
8. Update ADRs, godoc, implementation records, and run module and repository verification.

Because the repository uses nested Go modules, delivery proceeds inside-out:

```text
encounter provider → behavior consumer → session consumer → root documentation
```

Committed module dependencies must use published versions. Local `replace` or `go.work` overrides may assist development but will not reach CI.

## Discriminating toolkit proof

The double-door fixture will establish:

1. The monster sees Billy at the carpet cell.
2. Billy passes through the intervening space and behind the second door, breaking sight.
3. The monster retains Billy as held known-at-carpet Intel and receives no concealed live position.
4. No player is visible, so the monster moves to the exact carpet cell—even though Billy is elsewhere.
5. On arrival, Billy is absent from the monster's complete percept, so the encounter changes his position to unknown.
6. The next monster view contains Billy in neither `Seen` nor actionable `Remembered`, and the carpet cell is not pursued again.

A companion test will make another player visible during remembered pursuit and prove that visible behavior interrupts the ghost path on the next driver call.

The session proof will also assert persistence: a successful write stores held-unknown testimony, while a failed save retains the previous stored belief and publishes no correction.

## Smallest local-game observation

The closing local observation is the same behavior through the local dev dungeon:

- the monster sees the hero;
- the hero breaks sight through the second door;
- the monster walks to the last-seen carpet cell rather than toward hidden live truth;
- after checking the cell, it does not repeat that stale move.

That observation proves intentional behavior against the current encounter/session path. Finding the hero afterward is outside this slice.

## Explicitly not in this slice

- free-roam `Pump` correction;
- correction merely because a remembered cell becomes visible;
- sticky or persisted target commitment;
- route information-gain scoring;
- freshness or channel-reliability scoring;
- hiding, invisibility, sound physics, or deception behavior;
- searching after position becomes unknown; or
- a general personality/planning system.

The deception variant remains the next strong confirmation: false location testimony should drive the same pursuit-and-correction loop as a stale sighting without Intel consulting world truth.

## Toolkit artifacts

- `docs/ideas/monster-ghost-pursuit/design.md`
- `docs/ideas/monster-ghost-pursuit/plan.md`

Both are approved planning artifacts. Implementation has not started.
