# Run — 2026-11-24 — a creature hails for itself S2 a walk stops on a hail

processed: 2026-11-25T190427Z

Slice [rpg-project#1141](https://github.com/KirkDiggler/rpg-project/issues/1141) of story [rpg-project#1139](https://github.com/KirkDiggler/rpg-project/issues/1139). Base: `rpg-toolkit` `main` at [`74fc4e45`](https://github.com/KirkDiggler/rpg-toolkit/commit/74fc4e45).

## Shipped

- `MoveOutput.Hailed` and `MoveRest.Hailed`; a step whose `Hailed` is not empty stops the walk like a conversation's hail, `Addressed` untouched — [issue design 1](https://github.com/KirkDiggler/rpg-project/issues/1141).
- Site `testdata/guard-hails-walk.yaml` and `GuardHailsWalkSuite` playing the six steps with a reload after step 4 — [issue design 2 and 3](https://github.com/KirkDiggler/rpg-project/issues/1141).
- The encounter pin moves `v0.254.0` to `v0.255.0`; only that requirement line moves in `go.mod` — [issue pin](https://github.com/KirkDiggler/rpg-project/issues/1141).

## Decided

- Cells: captain on (1,4); alice starts (-3,4), four from him; bob starts (-3,3), five; alice's walk stops on (-1,4), bob's on (0,3); near cell (-1,5), far cells (-2,5) and (-1,3); distances asserted through `Encounter.Distance` in the test — [issue cells](https://github.com/KirkDiggler/rpg-project/issues/1141).
- `continuation.go` joined the files list for `MoveRest.Hailed` only, on the lead's ruling — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1141#issuecomment-7117486374).
- No test of a hail on a resumed (paced) walk: the pace fixtures have no creature that hails and a hall this size has no pace boundary, so none was added — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1141#issuecomment-7117486374).

## Found

- Rests on F1 to F5 all hold at `74fc4e45` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1141#issuecomment-7117404597).
- Gap: `MoveRest` (the second result carrying `Addressed`) lives in `continuation.go`, not listed by the issue — [gap](https://github.com/KirkDiggler/rpg-project/issues/1141#issuecomment-7117481999).
- Before `move.go` changed the test failed to build: `out.Hailed undefined (type *session.MoveOutput has no field or method Hailed)` — [issue step 4](https://github.com/KirkDiggler/rpg-project/issues/1141).
- Proofs (site file changed, then restored): no `scope` fails step 1 (`guard_hails_walk_test.go:163`, the walk is not stopped); no `not noted:greeted` fails step 6 (`:208`, alice stopped again); `within: 1` fails step 1 (`:167`, she stands on a different cell) — [issue step 6](https://github.com/KirkDiggler/rpg-project/issues/1141).
- Every act is delivered once per recipient, so the test reads one member's copy of the captain's words — [test](https://github.com/KirkDiggler/rpg-toolkit/blob/feat/a-walk-stops-on-a-hail/rulebooks/dnd5e/session/guard_hails_walk_test.go).
- `-race` run of the three suites: GuardHailsWalk 0.71s, KnownOnSightWalk 2.05s, SlippedAwayWalk 1.50s; whole module `go test ./...` ok in 60.3s; `golangci-lint` 0 issues — [issue commands](https://github.com/KirkDiggler/rpg-project/issues/1141).

## Mistaken

- The brief listed only `move.go` though the second result is in `continuation.go`; the builder reported it as a blocking gap — [gap](https://github.com/KirkDiggler/rpg-project/issues/1141#issuecomment-7117481999).
