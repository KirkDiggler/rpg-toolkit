# Run — 2026-11-22 — Status rounds lifetime S4 the session

Slice [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100) of story [rpg-project#1080](https://github.com/KirkDiggler/rpg-project/issues/1080). Base: `rpg-toolkit` `main` at [`7e19fae4`](https://github.com/KirkDiggler/rpg-toolkit/commit/7e19fae4).

## Shipped

- A giving with `until: { rounds: N }` is compiled with its end, arms one timer when fresh, and its lapse ends it; lapses are split out of the crossings before resolution sees them — PR (see the pull request linked from [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100)).
- Pins moved to `rulebooks/dnd5e v0.226.0`, `encounter v0.240.1`, `resolution v0.80.0`; baseline suite green after the pins and before any edit — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).
- Six new tests in `HoldStatusSuite` and one `CheckContent` test; no existing test edited — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).

## Decided

- The ending is written on the giving, where it is given — [operator, 2026-11-22](https://github.com/KirkDiggler/rpg-project/issues/989#issuecomment-7066217434).
- The session arms and clears because only the rules know a giving was fresh — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).
- Every fresh giving arms or clears, so a stale timer never ends a later giving; an already-held giving touches no timer — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).
- A lapse is split out of the crossings before resolution sees it, because resolution refuses an unknown kind and a refused verb is never saved — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).

## Found

- Rests on: F1 holds (three tags exist), F2 holds, F3 holds, F4 holds — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1100#issuecomment-7070184745).
- Without `ClearStatusLapse` all 13 hold tests still pass: an untimed status has no lapse part, so the stale event finds nothing — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1100#issuecomment-7070251554).
- The record has no beat for a status ending, so the order of the hold's end against a creature's step cannot be asserted; that test is left out — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1100#issuecomment-7070251825).
- Taking the lapse split out of `announcer.go` fails 5 tests, so that guard is proven by mutation — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).
- New test times under `-race`: AHoldForThreeRoundsEndsAfterThree 0.49s, ASecondGivingWhileHeldDoesNotRestartTheCount 0.42s, AHoldForRoundsSurvivesAReloadOfEveryStore 0.42s, AFightStillEndsAHoldForRoundsAndItsTimerFiresHarmlessly 0.40s, AHoldWithoutUntilAfterATimedOneIsNotEndedByTheOldTimer 0.61s, ALapseForAStatusThatAlreadyEndedChangesNothingAndTheGameGoesOn 0.50s, all under the 5 s target — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).
- `TestCastleShoulderingPast...` passes under `-race` (printed 0.00s) — [rpg-toolkit#2477](https://github.com/KirkDiggler/rpg-toolkit/issues/2477).

## Mistaken

- none observed — [rpg-project#1100](https://github.com/KirkDiggler/rpg-project/issues/1100).
