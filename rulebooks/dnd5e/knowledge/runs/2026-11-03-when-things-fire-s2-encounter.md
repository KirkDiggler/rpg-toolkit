# 2026-11-03 · When things fire S2 · Facing follows the legs

processed: 2026-11-05T201556Z

Story [rpg-project#825](https://github.com/KirkDiggler/rpg-project/issues/825), slice [rpg-project#829](https://github.com/KirkDiggler/rpg-project/issues/829), design R2. Encounter PR [rpg-toolkit#2252](https://github.com/KirkDiggler/rpg-toolkit/pull/2252).

## Shipped

- `walkPath` turns along a step unless the kind is displaced, at [`clocks.go`](https://github.com/KirkDiggler/rpg-toolkit/blob/aeaf8141/rulebooks/dnd5e/encounter/clocks.go#L1731). The `imposed` flag now means doors only.
- A held directive's hand-taken first cell turns under the same rule, at [`held.go`](https://github.com/KirkDiggler/rpg-toolkit/blob/aeaf8141/rulebooks/dnd5e/encounter/held.go#L146).
- Castle tests for Dissonant Whispers and Thunderwave on the captain, in `compelled_facing_test.go`.

## Decided

- The castle Thunderwave test walks the shove through `Direct` with the displaced kind, the seam `walkCastPushes` uses. Going through `RecordCast` forms the fight and the captain's own first turn turns it, which would hide the push's facing.
- No goldens moved. No golden in the module records a compelled facing.

## Found

- The moved beat carries no facing. Clients read facing from the member roster and from sight testimony, and the bard's sighting of the fled captain carries the new facing. Session and api need re-pins only.
- The web already turns compelled walks (`moveKind.ts`: own and compelled walk). The snap back the walk saw was the server's unturned facing arriving after the animation.
- The session suite is green against this encounter head through a local go.work.

## Mistaken

- `TestAPausedPushResumedKeepsItsFacing` pinned the old rule for compelled. It was re-pinned to face the flight, as design R2 rules.

— cross-team agent, on behalf of KirkDiggler
