# Run — 2026-11-23 — a note has a value S3a an author writes a note's value

processed: 2026-11-25T190427Z

Slice [rpg-project#1114](https://github.com/KirkDiggler/rpg-project/issues/1114) of story [rpg-project#1007](https://github.com/KirkDiggler/rpg-project/issues/1007). Base: `rpg-toolkit` `main` at [`7e91a697`](https://github.com/KirkDiggler/rpg-toolkit/commit/7e91a697f0879d3e56f0f2a6add32e66f981d97e).

## Shipped

- `give` takes `as`, lowered onto `Recognition.As` on a line's row, a conversation's outcome and a state's outcome — [rpg-project#1114](https://github.com/KirkDiggler/rpg-project/issues/1114).
- `noted` takes a plain word or `{ topic, as }`, lowered onto `TriggerRecognized.As`; `String()` reads it back — [rpg-project#1114](https://github.com/KirkDiggler/rpg-project/issues/1114).
- Three refusals at the place, in the engine's sentences, and `noted.as:<topic>` counts as asking the topic — [rpg-project#1114](https://github.com/KirkDiggler/rpg-project/issues/1114).

## Decided

- The values given are carried as a separate topic-to-value index (`topicValues`, built by `givenValuesOf`), beside the unchanged yes-or-no maps, because the issue left the carrying to the builder — [issue, refusals](https://github.com/KirkDiggler/rpg-project/issues/1114).
- The engine-topic and mixed-value refusals are one document walk (`topicValueRefusals`), not inside `givingOf`, so each lands at the give's own `.as` wherever it hangs — [issue, refusals](https://github.com/KirkDiggler/rpg-project/issues/1114).
- The engine's three sentences are copied into `dungeonspec` (they are unexported), with a subject word in place of the engine's `what` — [recognition.go](https://github.com/KirkDiggler/rpg-toolkit/blob/7e91a697/rulebooks/dnd5e/encounter/recognition.go#L404).
- `single_room_site.go` was also edited (not in the issue's list), to hand the grammar the value index — [issue, files](https://github.com/KirkDiggler/rpg-project/issues/1114).
- One edit under `readback/`: `"give": {"as"}` in `acknowledged` in `notyet_internal_test.go` — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1114#issuecomment-7089748159).

## Found

- Rests on F1–F8 all held at `7e91a697` — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1114#issuecomment-7089740616).
- Whole module suite before: all packages ok, 74.6s — [issue, step 1](https://github.com/KirkDiggler/rpg-project/issues/1114).
- An unknown key on a give in a whole document is refused earlier ("not a key this build reads"); `a give takes topic, as, …` shows only when `GiveSpec` is decoded alone, so the test decodes it so — [retired_words.go](https://github.com/KirkDiggler/rpg-toolkit/blob/7e91a697/rulebooks/dnd5e/encounter/dungeonspec/retired_words.go#L55).
- The read-back's walk does not list a state outcome's give keys; a line's give is listed — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1114).
- No committed picture and no hash pin moved; no existing test changed besides the one acknowledged key — [issue, allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1114).
- Proof: dropping `as` on a state's outcome fails the compiled assertion of the two-strikes test; whole suite at the final head, all ok; lint 0 issues — [issue, step 5](https://github.com/KirkDiggler/rpg-project/issues/1114).

## Mistaken

- The tests were written after the code, not before, so none was seen failing first; the mutant run is the only fail-proof — [build-slice](https://github.com/KirkDiggler/rpg-project/blob/main/.agents/skills/build-slice/SKILL.md).
- The first whole-suite run failed on the read-back's key guard; I reported it as a blocking gap before Amendment 1 allowed the edit — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1114#issuecomment-7089841859).
