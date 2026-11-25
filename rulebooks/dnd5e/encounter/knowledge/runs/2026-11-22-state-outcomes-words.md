# Run — 2026-11-22 — machine story S2 state outcomes words

processed: 2026-11-25T190427Z

Slice [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`8493b4dd`](https://github.com/KirkDiggler/rpg-toolkit/commit/8493b4dd).

## Shipped

- A behaviour definition's `outcomes:` is a site key: `BehaviorDefinitionSpec.Outcomes`, judged at `behaviorDefinitions.<key>.outcomes.<state>` and lowered onto every creature whose body machine it is, into the compiled creature's `Outcomes` — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- A topic a state gives counts as given for a line's `requires`, a post's `pass` and a machine's guard — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- The read-back says each one as a `routine.outcome` statement in the routine's group, with the note's connections and the state it is entered at — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).

## Decided

- A definition with `outcomes` attached to a creature but not as its `behaviorState` is refused at `<creature binding>.behaviors[i].use`, the path every other refusal about a binding's definition uses — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- The cause connection of a state's note is `{state, <state>, enters}`, the role word the conversation outcomes already use, so no new role word — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- No kind `routine.outcome` existed, so it is new (`git grep '"routine\.'` found state, wake, flag, row) — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- The read-back says "about" with the words of every beat that enters the state, joined with "or", since a state may be entered on more than one beat — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).

## Found

- F1 holds: `WhoInStateGiveTo` and `WhoInStateGiveAbout` at `whowords.go:61-62`, `WhoItself` at `:91`, table rows at `:101-102`, `Outcomes` at `behavior_binding.go:68`, `validateStateOutcomes` at `state_outcomes.go:26` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1104#issuecomment-7071808666).
- F2 holds (`behaviors.go:76`), F3 holds (`spec.go:1351`, `:1357`), F4 holds (`interactions.go:112`, `:181`, `:419`), F5 holds (`behaviors.go:96`, `:217`, `:307`, `:384`), F6 holds, F7 holds (`interactions.go:218`, `:241`), F8 holds (`routines.go:19`, `readback.go:134-139`, `noteedges.go:59`), F9 holds (`routinewords.go:116`), F10 holds (`contenthash_test.go:19`) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1104#issuecomment-7071808666).
- Failed first, each for its reason: the compile test (`Outcomes` nil), the play test (no note held), the topic test (the line and the guard refused the topic), the refusal table (no refusal at any path), the read-back test (no statement at the path) — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- Times with `-race`: compile 0.02s, play 0.03s, topic 0.07s, refusal table 0.16s, read-back 0.05s — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- Pin `castleContentHash`: old `4d35d5b81ae626c28b16aaed2ec7141aa76cd6074056e4f43abb28b6df5322a4`, new `2ad3019497e6fdc2f454941e9b4932a9e5e7f2fa6e22feae82e4650743a41d5c`; the compiled creature gained a field and no golden changed — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- `readback/notyet.go` was not in the files list and needs `"outcomes"` in its `behaviorDefinition` row — [gap](https://github.com/KirkDiggler/rpg-project/issues/1104#issuecomment-7071879248).
- `readback/folded.go` `foldRoutine` drops a `routine.outcome` from the folded text — [gap](https://github.com/KirkDiggler/rpg-project/issues/1104#issuecomment-7071901360).
- `apply_status` on a state's consequence is refused by the strict decode: `"apply_status" is not a key this build reads: they are arrest, give, jail, recognize, sceneMove` — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).
- Whole suite before any change: `go test ./... -count=1` green in 75s — [rpg-project#1104](https://github.com/KirkDiggler/rpg-project/issues/1104).

## Mistaken

- none observed.
