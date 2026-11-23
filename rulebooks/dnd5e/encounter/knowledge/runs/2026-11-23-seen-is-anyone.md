# Run — 2026-11-23 — a wake says whom it is about S2 seen is anyone

Slice [rpg-project#1125](https://github.com/KirkDiggler/rpg-project/issues/1125) of story [rpg-project#1123](https://github.com/KirkDiggler/rpg-project/issues/1123). Base: `rpg-toolkit` `main` at [`8b42fdd8`](https://github.com/KirkDiggler/rpg-toolkit/commit/8b42fdd8e986f49839cda0e71545bcd905d41227).

One line per entry. Every line ends with a link.

## Shipped

- `seen` lands for each member who comes into a listener's sight, in id order, the hostile check and the stop after the first gone (`landSightings`); the `stance` fact word, its reader, refusals and table row; the `seen` row's new words — branch `feat/seen-is-anyone`, [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- Not shipped: the read-back's sentence for `stance`, blocked by a reported gap — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).

## Decided

- Every choice here was handed by the issue; none was made by the builder — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).

## Found

- Rests on F1 to F10 all hold at `8b42fdd8` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1125#issuecomment-7094023730).
- Before the change: the nine tests of `seen_anyone_internal_test.go` failed, the landing ones with "does not contain" for the one who came into sight (no seen beat was landed for a non-enemy), the `stance` ones with 'fact "stance" is not in the vocabulary' at construction, the refusals and table ones with empty and old-words answers; the dungeonspec ones with the same vocabulary refusal — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- How someone comes into sight in a test: a Sight capability whose range for the guard the test sets (0 at first), then `refreshSight(rosterIDs())`; it lands the beats and does not match them — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- A hostile in sight forms a fight at once, and the world's pass (`spendWorldAction`) then does nothing, so the hostile tests match the landed beats with `matchLanded`. Every KindMonster and KindPlayer has a faction by default; `KindWorld` is the member with none (stance `none`) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- Two tests were rewritten after the change, the hostile and the none cases, for the two facts above; against the unchanged non-test files they failed on the vocabulary refusal — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- The three proofs, each from a clean tree and restored: the hostile check back fails known-on-sight, a stranger, not-hidden, a non-enemy is seen, the word's answers and the saved one; the `break` back fails known-on-sight, not-hidden and the non-enemy ones; the reader unknown for nobody fails the `not` test — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- Whole module suite before the change: all packages ok (new tests skipped). At the head: all ok but `TestStanceWords_AStepOnSeenSaysTheOneSawIsItsEnemy`, the gap's test — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- Existing tests changed, all of the allowed kinds: `TestMachineWords_FixedAndRequiredArePinned` (gains `stance.op=eq`), `TestMachineWords_ConditionsAreReadWhereTheTableSays` (a sample for `stance`), `TestMachineWords_LabelsArePinned` (the `seen` label and the `stance` row), `TestEveryConditionWordIsAConnection` via its samples in `readback/guardwords_internal_test.go` (a `stance` sample) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
- The read-back says a `stance` leaf through its default branch ("the creature running it's `stance` is exactly hostile"), because `guardWordsAt` in `dungeonspec/readback/routinewords.go` has no case for it — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).

## Mistaken

- none observed — [issue](https://github.com/KirkDiggler/rpg-project/issues/1125).
