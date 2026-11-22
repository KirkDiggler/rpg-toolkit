# Run — 2026-11-22 — machine story: a launched game carries state outcomes (session)

Slice [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`6483132d`](https://github.com/KirkDiggler/rpg-toolkit/commit/6483132d).

## Shipped

- `launch.go` hands a compiled creature's `Outcomes` to the engine beside its posts (`join.Outcomes = monster.placement.Outcomes`) — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- `go.mod`: `rulebooks/dnd5e/encounter` v0.240.1 to v0.243.0, the one requirement line that moved (`go.sum` follows) — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- `state_outcome_note_test.go`: `TestALaunchedCreatureNotesTheOneItHeardOnEnteringAState` (with the reload) and `TestALaunchedSiteMayRequireATopicOnlyAMachineGives` — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).

## Decided

- The plain line requires `noted: lied` (offered only after the note is taken), the first of the two forms the issue allowed — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- The line is read through `Manager.Afford` speak option rows, the session's own read of what a creature offers, as the castle tests do — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).

## Found

- F1 holds: launch.go:238 and :243 print, and `Outcomes` prints nothing in launch.go at base — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- F2 holds: the session go.mod printed `encounter v0.240.1`; F3 holds: `Outcomes map[string][]Consequence` at field.go:1084 and :1201 and compile.go:334 at v0.243.0 — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- F4 holds: only the two lines of F1 carry a compiled creature's state bindings; F5 holds: both pattern tests exist; F6 holds: state_outcomes_test.go has `outcomes:` sites — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- Baseline at base: `go build ./...` ok, `go test ./... -count=1` ok (session 58.7s) — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- `git diff origin/main -- rulebooks/dnd5e/session/go.mod` moves only the encounter line, v0.240.1 to v0.243.0 — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- With the new pin and no code change the whole suite passes, so no existing test changed (the chapel prayer in the castle broke none) — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- Both new tests failed first: the launch was refused with `launch: invalid encounter data` (the engine saw `noted: lied` required and nothing giving it); the one-line fix made both pass — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- Time with `-race`: the note test 0.11s, the launch test 0.03s — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
- Whole suite at the final head: session ok (59.1s), session-workbench ok; `golangci-lint run ./...` 0 issues — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).

## Mistaken

- The first site left out `arrangementDeclarations`, which the compile requires; the compile error named it and the fixture now carries it — [rpg-project#1106](https://github.com/KirkDiggler/rpg-project/issues/1106).
