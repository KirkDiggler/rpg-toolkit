# 2026-11-05 · The guard holds the gate, S2 · Session

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#842](https://github.com/KirkDiggler/rpg-project/issues/842), Task 7 of `ideas/the-guard-holds-the-gate/plan.md` (design R2 and R7). Session on encounter [rpg-toolkit#2278](https://github.com/KirkDiggler/rpg-toolkit/pull/2278).

## Shipped

- `go.mod` pins encounter at the pseudo-version of #2278's head 71b133ae, `v0.177.1-0.20261105184751-71b133aeeed3`, with no replace. It becomes the encounter tag before merge.
- Move reads the encounter's `Stopped` before `Stepped.To` ([`move.go:503`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/move.go#L503)). A stopped step names the walker's own cell at both ends, so reading it as a step walked her onto her own cell and on down the path. The stop sets [`MoveOutput.Barred`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/move.go#L115) (`barred`: `barrier`, `cell`, `reason`), `Status` stopped and `StopReason`. Nothing is paid, and the bump's intel, fight and ending ride the output.
- OpenDoor, CloseDoor and Unlock carry [`Refused`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/doors.go#L474) (`refused`: `holder`, `reason`) and return no error on a held door.
- Unlock still asks `DoorHeldFor` first, so a held lock throws no check and poses no Guidance pause. It now calls the verb anyway ([`doors.go:325`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/doors.go#L325)), and the encounter writes the bump.
- The `barred` beat folds into [`BarredAct`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/act_bodies.go#L324) (Mover, Barrier, Cell, Door, Verb), family act ([`barredPiece`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/act_family.go#L223)). A step's names the refused cell and no door. A door verb's names the door and verb and no cell. A beat missing a party or a place folds as undecoded ([`TestFold_Barred`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/act_fold_internal_test.go#L892), [`TestFold_BarredNamingNoPartyOrPlaceIsUndecoded`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/act_fold_internal_test.go#L918)).
- [`TestCastleTheBumpSpeaks`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/castle_gate_bump_test.go#L74): alice walks into posted gate-1. The Move is stopped with Barred naming him and his cell, she has not moved, and she still roams free. Her acts carry one BarredAct under her move, gate-1 `posted -> challenged`, and "Whoa there, buddy. Can I help you?". The second bump gets "I said, can I help you?".
- [`TestCastleTheHeldCrossingIsABump`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/castle_gate_bump_test.go#L110): a step across the opening's free cell is barred into the cell past the gate, with the same question.
- [`TestCastleAWallBumpWritesNothing`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/castle_gate_bump_test.go#L128): the gate wall stops the walk with no Barred, no BarredAct, no table answer, and gate-1 stays posted.
- [`TestCastleOpeningTheHeldDoorIsABump`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/castle_gate_bump_test.go#L144) and [`TestCastleAHeldLockedDoorIsAnsweredByTheGuard`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/castle_gate_hold_test.go#L266): OpenDoor and Unlock on the held gate answer Refused with the door unchanged, and write the bump naming the door and the verb. The unlock throws no d20.

## Decided

- The Move's field is `Barred`, the door verbs' is `Refused`, each copying the encounter's two shapes. One shared shape was the other cut, but a door refusal has no cell and a step has no door.
- `BarredAct` carries `Verb`, which the beat writes and rpg-api-protos v0.1.256's BarredAct has no field for. The api drops it until the proto grows one. Dropping it here would lose what the beat says.
- A paused Unlock answered through React after the door became held still returns `ErrDoorHeld` and writes no bump ([`TestAHeldDoorClosesAPausedUnlock`](https://github.com/KirkDiggler/rpg-toolkit/blob/a906f47c/rulebooks/dnd5e/session/held_unlock_resume_test.go#L62)). The answer is a reaction, not a new attempt, and React has no door output to carry a refusal. The next live Unlock in that test now answers `Refused`.
- The bump's audience is the encounter's. Every member's stream in the castle receives the BarredAct, gate-1's state change and his table answer, while `hears voice` reaches only those in earshot. The session filters nothing.
- A Move ending on the guard's own cell is still refused up front by ValidateWalkEnd as a bad position. Every bump test aims past him, as a click on the courtyard does.

## Found

- gate-1's line folds as a `table_answer` under a `time` act, separate from the bump's move act, with its text in `Say` and an empty `Word`. The api reads the say from there as it does for other picks.
- The castle dice counter counted gate-1's d100 table die as a roll. The held-unlock test now counts d20 checks, so it still proves no lock check was rolled.

## Mistaken

- None.

— cross-team agent, on behalf of KirkDiggler
