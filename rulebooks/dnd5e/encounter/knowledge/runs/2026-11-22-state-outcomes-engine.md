# Run — 2026-11-22 — machine story S1 state outcomes (engine)

processed: 2026-11-25T190427Z

Slice [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`35378f06`](https://github.com/KirkDiggler/rpg-toolkit/commit/35378f06b6b8c8d0013945bb52cb0ad1fc11fcfe).

## Shipped

- `MemberInput.Outcomes`, `JoinInput.Outcomes`, `CheckBehaviorsInput.Outcomes`, saved on `MemberData` and `ReserveData`, checked by `validateStateOutcomes` — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- A body machine step into a state does the `Recognize` outcomes bound to it, inside the same verb, after the `state` beat, told to the creature alone — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- Who-words `itself` at `state.give.to` and `the_one_named` at `state.give.about`; `grantedTopics` counts the topics a machine gives — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).

## Decided

- `behaviorStepped` in the `react` path is given `ev.Subject` (the one the beat names as the creature's senses named them), not `b.Subject` the issue names, because a landed sound carries its maker in `b.Subject` while naming nobody; a test fails on `b.Subject` — [gap comment on #1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- A state a transition enters on a beat not in the beat table is refused for `the_one_named` as naming nobody, because only a beat marked `Names` names someone — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- A state no transition enters is refused, the initial state included, because its outcomes could never run — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- Two rows were added to the who-word table (`itself`, `the_one_named` at the state places), so a line's row is not widened — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).

## Found

- Rests-on F1 holds: `behaviorStepped(` printed the definition and three calls, as written — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- F2 holds: `subject MemberID` at `behavior.go:260` and `subject: b.Subject` at `wakebeats.go:506` — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- F3 holds: `type Consequence struct` at `recognition.go:125`, seven kinds — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- F4 holds: seven lines printed at 224, 286, 355, 532, 572, 651, 742 — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- F5 holds: `the_one_named` at `whowords.go:81`, the table row at 95, `whoNow` at `whonow.go:56` — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- F6 holds: every `Posts` stop listed was found, and `Join` (`field.go` `JoinInput`) carries it too — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- F7, F8, F9, F10 hold; `itself` was no word in the engine package — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1103#issuecomment-7071543468).
- In the at-once heard path `b.Subject` is set only when the speaker was seen, so `b.Subject` and `ev.Subject` agree there; they differ only on a landed sound — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- The content hash pin did not move; no existing test changed — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- Every new test failed first because `Outcomes` and `WhoItself` did not exist, so the package did not compile: TestEnteringAStateNotesTheOneItHeard, TestEnteringAStateOnABumpNotesTheWalker, TestAHeardLineFromSomeoneUnseenNotesNobody, TestAStepBackIntoTheSameStateKeepsTheFirstNote, TestTheFirstStateGivesNothingAtSetup, TestAMachinesNoteIsToldToItsTakerAlone, TestALineMayRequireATopicOnlyAMachineGives, TestStateOutcomesSurviveSaveAndLoad, TestStateOutcomesAreRefused — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- With `-race` the new tests ran in 0.01s each, except TestStateOutcomesSurviveSaveAndLoad at 0.02s and TestStateOutcomesAreRefused at 0.04s — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- The whole module suite, `go test ./... -count=1`, passed before the change and at the final head; `golangci-lint run ./...` printed 0 issues — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).

## Mistaken

- The first refusal test used a recipe that steps back into its initial state, so the "nothing steps into it" case did not refuse; the test now uses a one-way recipe — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
- A test added later (TestAHeardNoiseLandedForTheRaiseNotesNobody) was needed because no issue test separated `ev.Subject` from `b.Subject`; the mutant now fails — [rpg-project#1103](https://github.com/KirkDiggler/rpg-project/issues/1103).
