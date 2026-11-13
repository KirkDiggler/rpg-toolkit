# 2026-11-05 · Click a hostile to attack, D4 · Encounter

processed: 2026-11-12T165555Z

Story [rpg-project#872](https://github.com/KirkDiggler/rpg-project/issues/872), decision D4 (the server answers "can I reach him this turn, and where do I stop"). Built on encounter v0.178.0 (main 6d8a753a), in commits [264cfd77](https://github.com/KirkDiggler/rpg-toolkit/commit/264cfd770e5676e3c0ec3ac85c84be7e54bf95f9), [59b274fb](https://github.com/KirkDiggler/rpg-toolkit/commit/59b274fb2c05e99d62d4e2258e739c1801458ce6) and the review fix on [#2285](https://github.com/KirkDiggler/rpg-toolkit/pull/2285).

## Shipped

- [`Encounter.Approaches`](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/approach.go) answers, per target, the cell a mover's own walk ends on within `RangeFeet` and in sight of it, within `Budget` steps or `Unbounded`. Each `Approach` row carries `Target`, `Reached`, `Cell` and `Path`. A target already in reach and seen is `Reached` with the mover's own cell and an empty path. It changes nothing.
- The walk is the player's: the mover's own picture gates a secret, a held crossing is closed, and a shut door is closed because a player's `Step` refuses it rather than opening it. The end is Standable and passes `ValidateWalkEnd`. Sight from the end is the one sight test (`sightReach.Reaches`, as `SeesWithin` asks it).
- `Approaches` is listed among the answers in [AGENTS.md](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/AGENTS.md).
- Tests: [approach_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/approach_test.go) and [approach_bench_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/approach_bench_test.go).

## Decided

- A budget is one bounded flood, however many targets are asked. Each target then reads only the cells within reach of it, cheapest first, ties by X then Y.
- No budget is at most one floor ([#2285 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2285)). Up to four targets are searched for one at a time (`searchTowardField`), each returning the flood's own cell and path by the rpg-project#762 promise. The first search that finds nothing has settled every cell the walk reaches, so its field is the unbounded flood and the targets after it read that field. Five or more targets take the one unbounded flood. `TestUnreachableTargetsCostOneFloor` pins that nothing floods after a failed search, and `TestManyTargetsTakeOneFlood` pins the one flood. `TestApproaches_CastleHoldsTheOracle` holds both branches to a brute-force unbounded flood on the castle, every monster at once and each alone.
- The end is not asked `ValidateWalkEnd` ([#2285 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2285)). The term survived its mutant because it is dead: that law refuses a cell where the mover sees a pass-through body, a current sighting's cell is that body's live cell, and the live fold already calls it PassThrough. The term and the `walkEndsRefused` refactor are removed. The oracle test still asks `ValidateWalkEnd` of every named cell.
- A target in reach must also be seen from the end cell. A cell across a wall from the target is within reach and sees nothing, and Attack's candidates are sight holdings.
- The fold is read live, as the step reads it. A body the mover has not seen still shapes the answer, because the walk would meet it and stop. The lead accepted the leak as a stated cost pre-playtest, and the doc comment says so.

## Found

- Benchmarks on the 10k keep with its 99-monster garrison, asked at five feet:

| read | first build ms/op | after review ms/op |
|---|---|---|
| fight, 6-step budget, all 99, one flood | 1.05 | 0.75 |
| free roam, one target, one search | 6.2 | 6.2 |
| free roam, all 99 targets | 156 (99 searches) | 55 (one flood) |

- The worst free-roam read is now three searches that each find a target, then one that finds nothing and settles the floor.
- Member seats in a `MemberInput` are authored offset pairs, while door edges and every reported cell are axial. The first fixture placed members by `cellAt` and built the wrong scene.
- Mutants killed: a walk that opens doors (the search and the flood each), no sight check, no budget, no cheapest-first sort, a pass-through end accepted, a search reach one cell too wide, searching every target, flooding again after a failed search, dropping the failed search's field.

— cross-team agent, on behalf of KirkDiggler
