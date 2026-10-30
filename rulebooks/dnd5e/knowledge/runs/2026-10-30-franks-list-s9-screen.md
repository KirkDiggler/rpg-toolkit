# 2026-10-30 · Frank's list S9 · The ending's result reaches the session seam

Story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), slice [rpg-project#733](https://github.com/KirkDiggler/rpg-project/issues/733). Contract [rpg-api-protos#408](https://github.com/KirkDiggler/rpg-api-protos/pull/408), web [rpg-dnd5e-web#1361](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1361).

## Shipped

- `session.ClockEntry.Outcome` (`EndingResult`: `success`, `failure`, empty) is folded from the ended beat's `outcome`. The empty value is an ending that declares none and every ended beat written before results existed.
- `ending_outcome_test.go` ends a run by a success, a failure and a no-result ending, and reads the closed run back through a fresh manager over the same stores.

## Decided

- The session names its own `EndingResult` rather than exporting encounter's, as it does for `AttackOutcome`.
- `session.Outcome` gains no `Result`. The screen reads the result from the ended clock entry only; no host reads `Outcome` for it, and the v1alpha1 wire `Outcome` has no field to carry it.
- No encounter bump: encounter v0.152.0, the session's pin, already writes `outcome` on the ended beat.

## Found

- The fold dropped the ended beat's `outcome` since rpg-toolkit#2155. The beat carried it, and no layer above the session could read it.

## Mistaken

- Nothing.

— cross-team agent, on behalf of KirkDiggler
