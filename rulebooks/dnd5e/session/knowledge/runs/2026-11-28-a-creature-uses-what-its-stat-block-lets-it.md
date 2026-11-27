# Run — 2026-11-28 — NPC behaviour 3c-1 a creature uses what its stat block lets it

Slice [rpg-project#1240](https://github.com/KirkDiggler/rpg-project/issues/1240) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`f15edd9a`](https://github.com/KirkDiggler/rpg-toolkit/commit/f15edd9a).

One line per entry. Every line ends with a link.

## Shipped

- `SheetFacts.Abilities` filled from `monster.AbilitiesOf` of the stored sheet (`abilityFactsOf`, sheets.go); a character states none — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- `strikerSeam.Activate` (activate_npc.go), wired as `Activator`: one Resolve of `resolution.NewAction` on the live encounter, the creature's sheet paying at the door, and a landing with Live AND Record that tells `activated` plus one `activation-result` per effect — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- `CheckContentOutput.Activations` and `ActivationRefusal`, rows walked by hand inside a group; `refuseLaunchContent` refuses the first as `ErrUnknownContent` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- The site `testdata/lieutenant-uses-second-wind-walk.yaml` and `activate_npc_test.go` (Tests 1 to 9) — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).

## Decided

- The landing follows the strike's (`attackLanding`: `&landing{Live: enc}` with a Record), not the run's (Live only) — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- The first-turn tests wound the Lieutenant by writing his stored hit points before his turn, not by scripting the fighter's blow: she is three cells away and only ends her turn, and his maximum is 9, so the roll plus 1 (11) always caps — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- Test 9 reads `monster.AbilitiesOf` of the Runner's stored sheet: no test in this module builds a `sheetSeam` to read `readFacts` directly, and Tests 1 to 3 prove the facts end to end — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- `convert_test.go` gains one `omitted` entry for `encounter.MonsterView.Abilities`, text from the amendment — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227722339).
- `template_lets_test.go` is not edited: `newLieutenantScene` already takes the compiled site, so the new tests read their own file — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).

## Found

- F1 to F5 hold at `f15edd9a`; F2 prints `mind/behavior v0.26.0` and `rulebooks/dnd5e v0.243.0`; `go mod tidy` moves only the four pins and 16 go.sum lines — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227714350).
- F7 failed at the pins: `TestConvertSuite/TestEveryInnerFieldIsCarriedOrJustified/MonsterView` ("encounter.MonsterView.Abilities is not carried ... and is not listed as a deliberate omission") — [gap](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227714350).
- F6: the misspelt row `{ activate: secnd wind }` LAUNCHES at the pins (err nil, one session stored), so the launch branch is reachable — [F6 answer](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227757434).
- F8: (a) no match for "Nothing makes a creature use what it has yet"; (b) no test file names Dasher or Disengager; (c) the NumField hits are projection tests, none over SheetFacts; (d) only template_lets_test.go:156,237 count `Templates`, unmoved — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227714350).
- His first turn, in order (one fighter EndTurn): turn-ended, answered, activated, activation-result, answered, moved, moved, answered, struck, heard, answered, stayed, turn-ended, tick — [Test 1](https://github.com/KirkDiggler/rpg-project/issues/1240).
- At full hit points (9 of 9): amount 0, requested 11 (1d10 at 10, plus 1), before 9, after 9. Wounded to 5: before 5, after 9, amount 4, requested 11 — [Tests 1 and 2](https://github.com/KirkDiggler/rpg-project/issues/1240).
- His sheet after: `Left["second-wind"]` 0, bonus action 0; after his next turn began the bonus action is 1 and `Left` still 0 — [Test 3](https://github.com/KirkDiggler/rpg-project/issues/1240).
- Her client is handed one `ActivateAct`: By member lieutenant, Ability Second Wind (ref dnd5e:features:second_wind), no Target, one `healing-applied` result targeting him; act verb `activate` — the shape of a player's (activation_events_test.go) — [Test 5](https://github.com/KirkDiggler/rpg-project/issues/1240).
- The refusal: a table row says `activate: secnd wind`, and the rules describe no ability by that word; it is one of second wind. A launch returns it inside `launch: no such content: ...` — [Tests 6 and 7](https://github.com/KirkDiggler/rpg-project/issues/1240).
- Proofs, each committed work restored with `git checkout`: (a) no Activator: Tests 1, 2, 3, 5 fail with no `activated` beat; (b) no Abilities in readFacts: the same four; (c) no Cost: he activates on every ask until the encounter's bound (15 `activated` beats in one turn), Tests 1 to 5 fail; (d) no Record: the same four fail at the beat assertion, which is checked before the sheet; (e) no descent into a group's rows: the bundle case fails; (f) no launch branch: Test 7 fails — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- Tests 4, 8 and 9 passed before the production change: they guard what must stay true (he still fights; a lacking creature does nothing; no kit lists nothing) — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
- Suite before (pins only): red on the one test above; after the omission entry: green (session 61.7s) — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227722339).

## Mistaken

- The first F6 scratch test matched `activate: second wind` twice, because the site's header comment says it too; the matches now use the row's braces — [F6 answer](https://github.com/KirkDiggler/rpg-project/issues/1240#issuecomment-7227757434).
- Proofs (b) and (d) first failed to compile (unused variable), which proves nothing; both were rerun as compiling mutants — [slice](https://github.com/KirkDiggler/rpg-project/issues/1240).
