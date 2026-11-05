# 2026-11-05 · The guard holds the gate, S2 · Encounter

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#842](https://github.com/KirkDiggler/rpg-project/issues/842), Task 6 of `ideas/the-guard-holds-the-gate/plan.md` (design R2, R7). Built on encounter v0.177.0 (main d161e7bd), in commit [0dc1196d](https://github.com/KirkDiggler/rpg-toolkit/commit/0dc1196d).

## Shipped

- [`BeatBarred`](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/beats.go) (`barred`) is written by [`Encounter.bump`](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/barred.go). Its payload is `mover`, `barrier` and, for a step, `cell`: the cell the step was refused into. For a door verb it carries `door` and `verb` (`open`, `close`, `unlock`) and no cell, matching rpg-api-protos#418's `BarredAct`.
- A player's [`Step`](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/step.go) refused by a body answers with output. `StepOutput.Stopped` is a [`StepStop`](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/field.go) with `Barrier`, `Cell` and `Reason`. Nothing moves and no pace is spent. The output carries the bump's `Seq`, `IntelDeltas`, `Formed` and `Outcome`.
- `OpenDoor`, `CloseDoor` and `Unlock` on a held door answer with `Refused`, a [`DoorRefusal`](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/barred.go) with `Holder` and `Reason`. The door is unchanged and `State` is its current state.
- The bump is matched on the barrier alone, through `matchOne`, and is never landed. A held crossing's barrier is the nearest holder the mover sees, else the nearest holder.
- `barred` is a scoped beat in the machine vocabulary, `MachineBeats` and `MachineBeatScoped`.
- A fight formed by a stance turn is now kept in `stanceFormed` ([turning.go](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/turning.go)), so a bump's output reports it.
- Castle mirror: gate-1's `posted` and `alert` gain `on: barred` to `challenged` with `pick: barred`. `challenged` holds the gate, answers a second bump with `pick: challenged`, and returns to `posted` after three rounds. `gate-patrol` gains both keys.
- Tests: [barred_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/barred_test.go) and [castle_king_bump_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/castletest/castle_king_bump_test.go).

## Decided

- The barrier's pick reopens the bump's act (act.go, Law 2: a reaction reopens its trigger's id). Kirk's walk on :3031 showed his answered beat and his voice under a fresh `time:` act (record seq 169-170), so the web could not tie his line to the bump. `bump` now sets `reactAct` for the match, and `runIntents` reopens it for the pick and its intent. A clock-raised pick still mints `time:`. [`TestTheAnswerIsInTheBumpsAct`](https://github.com/KirkDiggler/rpg-toolkit/blob/hold-the-gate-s2-encounter/rulebooks/dnd5e/encounter/barred_test.go) asserts one correlation for the bump, his state, his answer and his voice, for a step and a door verb, and fails without the fix. The replay harness opens no acts, so its bumps mint `time:` as before and still replay.
- A door bump's `door` is the qualified door id (`the-mad-king/gate-wall-door`), the same id the door verbs take and the `door` beat writes.
- `alert` also answers a bump, beside `posted`. An alert guard on his post still holds the gate.
- The refresh after a bump runs only when the reaction picked or turned a body's blocking. A bump nobody answers writes the one beat.
- A door bump's scope is measured from the door's cell nearest the actor. That cell is not written on the beat.
- Door outputs carry no `Outcome` field, as before. A host reads a closed run as it does after any door verb.

## Found

- The session asks `DoorHeldFor` before Unlock rolls, and refuses on its own. Until Task 7 calls the verb on a held door, a live Unlock attempt writes no bump.
- The session's OpenDoor and CloseDoor still translate `encounter.ErrDoorHeld` into its held-door error ([session/read.go:728-736](https://github.com/KirkDiggler/rpg-toolkit/blob/d161e7bd/rulebooks/dnd5e/session/read.go#L728-L736)). After the re-pin, a held open or close returns a nil error with `Refused` set, so that translation never fires: the session must read `Refused` off the output.
- After the re-pin, the session's `Move` must read `Stopped` before `Stepped.To`. A bump's `Stepped` reports the mover's own cell at both ends.
- Replay re-issues a bump from the record ([replay_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/0dc1196d/rulebooks/dnd5e/encounter/replaytest/replay_test.go)): a step bump as a Step to its cell, a door bump as OpenDoor.
- Scope tests ([#2278 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2278#pullrequestreview-5790410979)): a holder three hexes off ignores `within: 1` and answers `within: 3`; a door bump is measured from the door's cell nearest the actor. Each fails with its mutant: a bump matched with no cell, a door bump measured from the farthest door cell, or from the actor's own cell.
- The castle bump test pins `challenged` returning to `posted` after three rounds; changing the castle's `rounds: 3` to 30 fails it.
- Mutants killed: landing the bump instead of matching it, matching everything landed, skipping the refresh, dropping the stance fight, the wrong barrier, no door field, a bump that spends pace, `barred` unscoped, `barred` unlisted.

— cross-team agent, on behalf of KirkDiggler
