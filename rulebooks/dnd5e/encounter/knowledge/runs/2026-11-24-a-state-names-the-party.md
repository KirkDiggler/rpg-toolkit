# Run — 2026-11-24 — a state names the party S1 note about party_in_sight and the_gone

Slice [rpg-project#1135](https://github.com/KirkDiggler/rpg-project/issues/1135) of story [rpg-project#1134](https://github.com/KirkDiggler/rpg-project/issues/1134). Base: `rpg-toolkit` `main` at [`2d7824ec`](https://github.com/KirkDiggler/rpg-toolkit/commit/2d7824ec).

One line per entry. Every line ends with a link.

## Shipped

- A machine state's note may be about `party_in_sight` or `the_gone`, resolved from the one the beat names; the who-word table, resolver, engine check, author's file check and read-back all say it — [slice](https://github.com/KirkDiggler/rpg-project/issues/1135).

## Decided

- The group is read from `partyOf(named)`, the creature itself is never in it, and a beat that names nobody gives nobody, as the issue says — [design](https://github.com/KirkDiggler/rpg-project/issues/1135).
- The refusal for another word lists the three words in double quotes; the naming-beat sentence says "notes about `<word>`" — [design 4](https://github.com/KirkDiggler/rpg-project/issues/1135).
- The read-back says a non-`the_one_named` note as "about <label> (the one the beat names is <who>)" — [design 6](https://github.com/KirkDiggler/rpg-project/issues/1135).

## Found

- Rests-on F1 to F8 all held at `2d7824ec` (first comment on the slice) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1135#issuecomment-7115367680).
- Before the change every new test failed: engine tests with "member cannot be seated" (about "party_in_sight": the subject is "the_one_named"), the check tests for lacking the words; dungeonspec and read-back tests with the old "the subjects are `the_one_named`" refusal — [slice](https://github.com/KirkDiggler/rpg-project/issues/1135).
- Proofs: counting every member of the party as seen fails the party-he-sees, gone, both, creature-in-neither and saved tests; counting nobody as seen fails six tests; the_gone skipping the sight-record half fails the gone, both, creature-in-neither and saved tests — [slice](https://github.com/KirkDiggler/rpg-project/issues/1135).
- Existing tests changed: `TestStateOutcomesAreRefused` naming-beat sentence (state_outcomes_internal_test.go) now says "notes about `the_one_named`"; dungeonspec `state_outcomes_test.go` "a subject that is not the one the beat names" lists the three words — [slice](https://github.com/KirkDiggler/rpg-project/issues/1135).
- Whole module suite green before and at the final head (`go test ./... -count=1`), lint 0 issues — [slice](https://github.com/KirkDiggler/rpg-project/issues/1135).
- A sight refresh keeps the record of someone who went out of range, so intelLog.On is the "saw earlier" test — [arrest.go](https://github.com/KirkDiggler/rpg-toolkit/blob/2d7824ec/rulebooks/dnd5e/encounter/arrest.go#L471).

## Mistaken

- none observed — [slice](https://github.com/KirkDiggler/rpg-project/issues/1135).
