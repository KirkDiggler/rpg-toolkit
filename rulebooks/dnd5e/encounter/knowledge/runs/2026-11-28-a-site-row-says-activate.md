# Run — 2026-11-28 — NPC behaviour 3b-2 a site's table row says activate

Slice [rpg-project#1234](https://github.com/KirkDiggler/rpg-project/issues/1234) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`a84644f5`](https://github.com/KirkDiggler/rpg-toolkit/commit/a84644f5).

## Shipped

- `AnswerSpec.Activate`, its decode, grammar and compile (`answerOf`), carried by `answerWord`, `validateEntry`, `AnswerData` (both directions) and `completionOf`; said by the site check's long and short sentences with an `ability`/`activates` connection — [rpg-project#1234](https://github.com/KirkDiggler/rpg-project/issues/1234).
- The pin of `mind/behavior` to v0.26.0; `go mod tidy` moved only the `mind/behavior` line and its two `go.sum` lines — [pin commit](https://github.com/KirkDiggler/rpg-toolkit/commit/078d2549).

## Decided

- The shape of the word is judged in two places: a value that is not a string (a mapping, a list, a number) is refused at the node in `AnswerSpec.UnmarshalYAML`, since the decoder would otherwise refuse it in its own voice; an empty or blank string is refused in `grammar.answerEntry`, before a `use:` row is set aside, with path `.activate` and the line — [issue Design 2](https://github.com/KirkDiggler/rpg-project/issues/1234).
- The word is trimmed in `answerOf` and nothing else is done to it; no list of ability words, the host's content check judges it (design law C1) — [ADR-0051 point 6](https://github.com/KirkDiggler/rpg-toolkit/blob/main/docs/adr/0051-a-driven-turn-can-activate-an-ability.md).
- The castle's content hash pin is re-pinned to `89d972b7...3a7`: `canonicalEncode` walks every struct field by name, so the new compiled field moves it though the JSON picture does not move — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1234#issuecomment-7227076213).
- `tabledriver.go`, `clocks.go`, `facts.go` and `turndriver.go` are untouched; the row is absent from every roll until `Facts.CanActivate` is filled — [issue "Leave alone"](https://github.com/KirkDiggler/rpg-project/issues/1234).

## Found

- F1 to F6 hold; F7 did not hold: the pin alone moved `TestTheCastlesContentHashIsPinned` from `e337704d...32ae6` to `89d972b7...3a7` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1234#issuecomment-7227064913).
- F8 hits: `spec.go:3026`/`:3048`, `answer.go:722`, `readback/notyet.go:61` edited; `readback/notyet_internal_test.go:80` the allowed test edit; `ally_internal_test.go:109`, `dungeonspec/then_test.go:117` (words taking a selector) and `knowledge/decisions.md:158` (the `answered` beat) not edited — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1234#issuecomment-7227064913).
- `cloneAnswer` starts from `out := entry`, so the string is copied; nothing was added — [`answer.go` cloneAnswer](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/answer.go).
- A saved encounter holds a member's table as mind/behavior's own rows (JSON key `Activate`, omitted when empty); `AnswerData` is the flat table and an option's rows (`"activate"`, omitted when empty) — [`data.go`](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/data.go).
- A `behaviorDefinitions` table is decoded as `behavior.Answer` directly, so `activate:` was already writable there at v0.26.0; the placement's `time` rows go through `AnswerSpec` — [`dungeonspec/behaviors.go`](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/dungeonspec/behaviors.go).
- A bundle's `$name` parameter stands for the word today: `activate: $word` with `with: { word: second wind }` compiles to a row holding `second wind`; not built, found — [bundles_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/dungeonspec/bundles_test.go).
- Refusals: `activate: ""` and `"   "` print `place[18].on.time[0].activate: `activate` names one ability by its word, as a stat block's `lets` writes it: `activate: second wind` (line 171)`; a mapping or a list print `line 171: ` plus the same sentence; two words print `an entry does one thing: `attack` and `activate` in the same entry is 2 (line 171)`; under `success` `...activate: `activate` is what a creature does with time, and `success` is an outcome`; on a group `a group carries `activate`; a group's answer is its rows, so the word goes on a row inside it (line 1)` — [activate_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/dungeonspec/activate_test.go).
- Proofs: (a) no `Activate` in `answerOf` fails Tests 1, 3, 5, 8 and 11 but NOT Test 7, which saves hand-built rows and never goes through `answerOf`; (b) `activate` out of `entryWords` fails Tests 1, 3, 4 and 5 (the row "does nothing and says nothing"); (c) `omitempty` removed fails the `AnswerData` no-key assertion; (d) the said key removed fails `TestEveryKeyIsClassified` and Test 11 — [PR](https://github.com/KirkDiggler/rpg-toolkit/pulls).
- The suite passes at the pin before any other change in every package except the hash test; at the final head every package passes, `go build`, `go vet` and `golangci-lint run ./...` are clean — [PR](https://github.com/KirkDiggler/rpg-toolkit/pulls).

## Mistaken

- The tests were written after the code, not before it, so "seen to fail first" is shown by the proofs instead; the first proof (a) was a build failure and was redone — [PR](https://github.com/KirkDiggler/rpg-toolkit/pulls).
- Test 11 first read a `behaviorDefinitions` table, which never goes through `answerOf`, so proof (a) did not bite; it now reads a placement's table — [readback activate_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/dungeonspec/readback/activate_test.go).
