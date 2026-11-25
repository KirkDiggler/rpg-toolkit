# Run — 2026-11-24 — the one who slipped away S2 walk site and test

Slice [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136) of story [rpg-project#1134](https://github.com/KirkDiggler/rpg-project/issues/1134). Base: `rpg-toolkit` `main` at [`9bb217a9`](https://github.com/KirkDiggler/rpg-toolkit/commit/9bb217a9).

## Shipped

- `testdata/slipped-away-walk.yaml` and `slipped_away_walk_test.go` (`SlippedAwayWalkSuite`: `TestTheOneWhoSlippedAway`, `TestNobodySlippedAway`), with a reload after step 3 — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).
- `go.mod` pins `rulebooks/dnd5e/encounter` at `v0.254.0` (only that line moved from `v0.252.0`), `go.sum` tidied — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).

## Decided

- The aside cell is `(-6,7)`, four steps out from the doorway along row 7, because it is unseen from his start `(-1,4)` and from his door spot `(-4,6)`; the seen cell `(-3,7)` is seen from the door spot; alice comes back to the doorway `(-3,6)` because bob stands on `(-3,7)` — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1136#issuecomment-7115737005).
- The captain's start was not moved; the first option (a yard cell) worked — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1136#issuecomment-7115737005).
- The baseline at `origin/main` was not run before the pin moved; `KnownOnSightWalk` passing at the pin stands in for it — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1136#issuecomment-7115737005).

## Found

- Rests on F1 to F4 all hold at `9bb217a9` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1136#issuecomment-7115674383).
- The old aside cell `(-4,7)` is seen from his start `(-1,4)` and not seen from his door spot; `(-3,7)`, `(-4,7)`, `(-5,7)`, `(-5,8)`, `(-4,8)` are seen from the start — [gap](https://github.com/KirkDiggler/rpg-project/issues/1136#issuecomment-7115727899).
- Seen from neither cell: `(-6,7)`, `(-7,7)`, `(-8,8)`, `(-7,8)`, `(-8,9)`, `(-9,10)`; seen from the door spot only: `(-3,8)`, `(-2,8)`, `(-1,8)`, `(0,8)`, `(-1,9)`, `(-3,9)`, `(-3,10)` — [the test's comment](https://github.com/KirkDiggler/rpg-toolkit/blob/test/slipped-away-walk/rulebooks/dnd5e/session/slipped_away_walk_test.go).
- Bob reaches `(-6,7)` in one walk with no Waits; a walk may pass through an ally's cell but may not end on it ("move destination: bad position") — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).
- Proof 1: taking the `the_gone` give out makes the site fail to compile ("nothing gives it that value" for `wanted`), so both tests fail at load; pointing it at `the_one_named` instead compiles and "There is the other one." fails — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).
- Proof 2: `about: the_one_named` for `party_in_sight` fails `TestNobodySlippedAway` (bob keeps `the-way-in`) — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).
- Proof 3: taking the `on: seen` step out fails "There is the other one." — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).
- `go test -race -run 'SlippedAwayWalk|KnownOnSightWalk'` passes; the new suite takes 1.51s, the known-on-sight suite 2.07s — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).
- `go test ./... -count=1 -timeout 20m` passes (session 60.6s); `golangci-lint run ./...` prints 0 issues — [rpg-project#1136](https://github.com/KirkDiggler/rpg-project/issues/1136).

## Mistaken

- The brief said the four cells "hold here"; the aside cell had only been checked from the door spot, so step 1 failed until it was found again — [gap](https://github.com/KirkDiggler/rpg-project/issues/1136#issuecomment-7115727899).
