# Run — 2026-11-09 — tests under control S2: the three oracle tests prove their equality on a small keep

processed: 2026-11-12T165555Z

Slice [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909) of story [rpg-project#907](https://github.com/KirkDiggler/rpg-project/issues/907). Base: `rpg-toolkit` `main` at [`b7ac2a42`](https://github.com/KirkDiggler/rpg-toolkit/commit/b7ac2a42). Test code only; no production file changed.

## Shipped

- `smallKeepPlan` is the benchmark keep's own plan at the smallest size its doors allow: all 29 rooms, the courtyard, both grounds and all 36 footprint doors in 1,230 hexes. Every room is 5 wide and 3 deep; the hidden stash and the tower are 3 wide; the larder is 6 deep; the grounds are 3 across. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- `TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans` holds all 36 doors of the small keep to the whole-floor scan. The 10k garrison comparison is `BenchmarkGarrisonDoorOpeningsAgainstScan`. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- `TestPlacedReach_AgreesWithTheWalk` runs a castle row and a small keep row. The 10k comparison is `BenchmarkPlacedReachAgreesKeep10k`. The lane sampler draws a lane's far end uniformly from the same hex disk by rejection from a square, instead of listing the disk for every lane. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- `TestObserversOf_EqualsFullRefresh` keeps the castle row and moves the keep row to the small keep's garrison, with sight scaled to the keep. The 10k garrison row is `BenchmarkObserversOfEqualsFullRefreshKeep10k`. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)

## Decided

- The 10k comparisons live in benchmarks, not behind an environment guard. CI runs `go test -race ./...` and no benchmark, so a benchmark is never a gate. It is reachable by name with `go test -run '^$' -bench <Name> -benchtime 1x .`, and it fails if the equality breaks. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- The door test opens the small keep with two players and no garrison. A door's crossings depend on floor and placement only, and the garrison's monsters cost the open. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- The observers keep row is a garrison of four monsters per 100 hexes with one patrol, seen at 8 cells. At the 10k keep's density and sight 24, nobody in 1,230 hexes stood one cell past a door's sight, and the row no longer failed the bare-sight mutant. A patrol's route rarely fits a 3-deep room, so only one walks. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- The observers keep row stays out of `-race` runs, as the 10k row was. Under `-race` it costs 8 s, and `TestDoor_RefreshesTheRimObserver` already fails the same mutant there. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)

## Found

Commands, from `rulebooks/dnd5e/encounter`, with the box loaded (load average 4.5 to 9):

```
go test -race -count=1 -run 'TestObserversOf_EqualsFullRefresh$|TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans$|TestPlacedReach_AgreesWithTheWalk$' -v .
go test -race -count=1 -timeout 30m .
go test -count=1 -run '^$' -bench 'BenchmarkGarrisonDoorOpeningsAgainstScan|BenchmarkPlacedReachAgreesKeep10k|BenchmarkObserversOfEqualsFullRefreshKeep10k' -benchtime 1x .
```

| test, under `-race` | before | after |
|---|---|---|
| `TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans` | 26.6 s | 1.6 s |
| `TestPlacedReach_AgreesWithTheWalk` | 25.2 s | 2.6 s |
| `TestObserversOf_EqualsFullRefresh` | 25.8 s | 25.2 s |
| root package, test time (wall) | 150.5 s (179.4 s) | 112.5 s (113.6 s) |

- The door test's time was the open: 24 s of opening the 99-monster garrison under `-race`, against 2.3 s of scan. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- The reach test's oracle spent 4.1 of its 4.3 s listing a radius-25 disk per lane to draw one end, so the castle row was as slow as the keep's. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- The observers test does not shrink under `-race`. Its 25 s is the castle row; the keep row was already skipped there. The castle brings a posted guard, two arrivals and eight creatures on a table alone, which no generated keep has, so it cannot be swapped for a keep without losing cases. It is left for the shard ([rpg-project#908](https://github.com/KirkDiggler/rpg-project/issues/908)) and the budget allowlist (slice 3). Without `-race` the keep row fell from about 15 s to 1.0 s. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- The root package is not under 60 s. The observers castle row and the rest of the package remain. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- All three benchmarks pass at 10k: 0.27 s door scan, 0.07 s reach, 10.1 s observers row, each after its keep opens. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)

Mutation checks, each applied alone and restored:

- `placed_props.go`, `field.doorCrossings`: `o, pad := d.placement.Origin, 2*float64(placedPlaneCellWidth)` became `o, pad := d.placement.Origin, 0.0`. The small door test fails, with 15 doors differing from the scan. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- `placed_reach_index.go`, `placedReach.near`: `x0, x1 := max(s[0], r.minB[0]), min(e[0], r.maxB[0])` became `x0, x1 := max(s[0], r.minB[0]), min(s[0], r.maxB[0])`. Both the castle and the small keep rows fail. Skipping `r.always` fails the castle row only, since the keep has nothing movable. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- `encounter.go`, `Encounter.observersWithin`: `float64(sight[id]+1)` became `float64(sight[id])`. The small keep row fails at move 6 and the 10k benchmark at move 10. The castle row passes it; under `-race`, `TestDoor_RefreshesTheRimObserver` fails it. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- Two mutants survive at every size, including 10k: a zero reach margin in `placedReach.near`, and a step's cells dropped from its refresh. `appendMovementBeat` notes the same two cells, so the refresh's own list is redundant. The old comment's claim that the 10k row failed "a step's cells dropped" no longer reproduces. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)

## Mistaken

- The first small observers garrison kept the 10k keep's density and sight. It passed the bare-sight mutant that the 10k row fails, so it had lost the rim case. Only the mutation check showed it. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
- Raising the garrison with every fifth monster on patrol failed to compile the keep: "no route of floor cells fits" in a 3-deep room. — [rpg-project#909](https://github.com/KirkDiggler/rpg-project/issues/909)
