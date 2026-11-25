# Run — 2026-11-23 — a wake says whom it is about S1 the topic beat names whom

processed: 2026-11-25T190427Z

Slice [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124) of story [rpg-project#1123](https://github.com/KirkDiggler/rpg-project/issues/1123). Base: `rpg-toolkit` `main` at [`e6035dc5`](https://github.com/KirkDiggler/rpg-toolkit/commit/e6035dc5438ff614fe19e7209b2ef93337f36f5e).

## Shipped

- One `topic` beat for each subject whose note changed (first note or replaced value), naming that subject; `namesSubject` answers true for it; the `topic` row gains `Names` and `About` — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).
- Rests-on F1 to F10 all held at `e6035dc5`: F1 one line (`wakebeats.go:290`), F2 two lines, F3 `namesSubject` as described, F4 one line (`machinewords.go:441`), F5 two lines (`:291`, `behaviors.go:738`), F6 one line (`:622`), F7 one line (`recognition.go:933`) and two in `perception.go`, F8 three lines, F9 prints `1`, F10 seven lines — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1124#issuecomment-7093679197).
- The whole module suite before any change: every package `ok`; at the final head: every package `ok` — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).

## Decided

- Each test in the new files failed before the change: the five tests that build a machine failed with "`noted.as:standing` asks about the one this beat is about, and `topic` is about nobody: it is read on heard, seen, barred and approached: member cannot be seated"; the words test failed with the same sentence; the table test and the `names its subject` row failed with `Should be true` and `expected "alice", actual ""`; the dungeonspec tests failed with "`topic`, which names nobody"; the read-back test failed at compile with the same refusal — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).
- Proof one, `beatTopic` out of `namesSubject`: "the note wakes him" and "saved between the give and the wake" fail — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).
- Proof two, one beat only (a `break` after the first changed subject): "one beat for each one whose note changed" fails — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).
- Proof three, a beat for every subject (`if true`): "a confirmed note is no news for that one" fails, and so does the existing `TestNoteValueTheSameValueGivenAgainLandsNoTopicBeat` — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).
- Existing tests changed: `heard_line_internal_test.go` (three sentence lists gain `topic`; the `topic` example row now uses `time`, since the `fact` row already exists; two rows added), `note_value_step_internal_test.go`, `state_outcomes_internal_test.go` and `dungeonspec/state_outcomes_test.go` (sentence lists gain `topic`), `dungeonspec/heard_words_test.go` (the case now rewrites the castle captain's `on: fact` step on `fact:gate-cowed` at `ready.transitions[1]`), and `machinewords_internal_test.go` (two pinned lists gain `topic`, reported as a gap) — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1124#issuecomment-7093869753).
- The tests that need to see `enc.landed` give through `holdTopic` and not through `Speak`, reported as a gap — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1124#issuecomment-7093869407).

## Found

- A `Speak` runs its own wake pass: the beats a give lands are matched inside the same call, so `enc.landed` is empty right after it — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1124#issuecomment-7093869407).
- `TestMachineWords_WhereAllowedIsPinned` pins the naming beats and the beats `noted:` is read on, so a change to the table's `Names` column breaks it by design — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1124#issuecomment-7093869753).
- No struct field was added: the castle's compiled picture and hash pin did not move, and every castle test passes unchanged — [rpg-project#1124](https://github.com/KirkDiggler/rpg-project/issues/1124).

## Mistaken

- The tests were first written against `Speak` and `enc.landed` as the issue words them; the false premise showed only once the implementation ran, after the change had been made. The rewritten tests were then checked failing against the unchanged non-test files, and the three proofs ran on the committed head — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1124#issuecomment-7093869407).
