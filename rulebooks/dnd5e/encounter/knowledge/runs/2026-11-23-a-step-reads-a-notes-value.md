# Run — 2026-11-23 — a note has a value S2 a step reads a note's value

Slice [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113) of story [rpg-project#1007](https://github.com/KirkDiggler/rpg-project/issues/1007). Base: `rpg-toolkit` `main` at [`20a4225e`](https://github.com/KirkDiggler/rpg-toolkit/commit/20a4225e6d9edd9fa1e6c8ae9f9a0f3013be8da9).

## Shipped

- The fact word `noted.as:<topic>` (string, `eq`): its constant, `MachineNotedAs`, reader, registration for every topic, words-table row and `the_one_named` who-word place — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- Its refusal on a beat that names nobody, in `noted:`'s sentence — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- A value that replaces the held one lands the `topic` beat; the same value again lands none — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- The two-strikes machine as an engine test, with save and load mid-way — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).

## Decided

- The reader always answers known and answers the empty string for nothing noted, no value, or a beat naming nobody, because an unknown fact makes the step depend and a `not` would never fire (F7) — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- "Replaced" is tracked in `holdTopic`'s own loop where the held value is compared with the new one, not inferred from the store's output, as the issue said — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- The word's row, label and who-word place follow the issue's text exactly; no template gap, since `{id}` and `{value}` are both plain slots — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).

## Found

- Rests on: F1 to F8 all held at `20a4225e` — [Rests on comment](https://github.com/KirkDiggler/rpg-project/issues/1113#issuecomment-7089562312).
- Whole module suite before any change: every package `ok` (encounter 57.4s, 74.7s wall) — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- Whole module suite at the final head: every package `ok` (71.5s wall) after the four listed test edits — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- Proof 1: a reader that answers unknown when nothing is noted fails `TestNoteValueStepANotOfAnUnnotedValueIsKnownAndApplies`; restored — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- Proof 2: dropping the "replaced" half of the beat rule fails `TestNoteValueAReplacedValueLandsTheTopicBeat`; restored — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- Existing test changed on purpose: `TestNoteValueAReplacedValueLandsNoTopicBeat` is now `TestNoteValueAReplacedValueLandsTheTopicBeat`, the old rule's pin — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- Existing tests that list every published word, each updated with the new word: `TestMachineWords_FixedAndRequiredArePinned` (`machineargs_internal_test.go`), `TestMachineWords_ConditionsAreReadWhereTheTableSays` and `TestMachineWords_LabelsArePinned` (`machinewords_internal_test.go`), `TestEveryConditionIDResolves` and `TestEveryConditionWordIsAConnection` (`dungeonspec/readback/guardwords_internal_test.go`, one sample map) — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
- No non-test file beyond the listed ones was touched; no picture or hash pin moved — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).

## Mistaken

- The first mutation of the "replaced" half removed a use and so did not compile; the mutant was redone as `replaced = false` — [rpg-project#1113](https://github.com/KirkDiggler/rpg-project/issues/1113).
