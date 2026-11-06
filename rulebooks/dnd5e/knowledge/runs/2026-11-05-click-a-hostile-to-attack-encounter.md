# 2026-11-05 · Click a hostile to attack, D4 · Encounter

Story [rpg-project#872](https://github.com/KirkDiggler/rpg-project/issues/872), decision D4 (the server answers "can I reach him this turn, and where do I stop"). Built on encounter v0.178.0 (main 6d8a753a), in commits [264cfd77](https://github.com/KirkDiggler/rpg-toolkit/commit/264cfd770e5676e3c0ec3ac85c84be7e54bf95f9) and [59b274fb](https://github.com/KirkDiggler/rpg-toolkit/commit/59b274fb2c05e99d62d4e2258e739c1801458ce6).

## Shipped

- [`Encounter.Approaches`](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/approach.go) answers, per target, the cell a mover's own walk ends on within `RangeFeet` and in sight of it, within `Budget` steps or `Unbounded`. Each `Approach` row carries `Target`, `Reached`, `Cell` and `Path`. A target already in reach and seen is `Reached` with the mover's own cell and an empty path. It changes nothing.
- The walk is the player's: the mover's own picture gates a secret, a held crossing is closed, and a shut door is closed because a player's `Step` refuses it rather than opening it. The end is Standable and passes `ValidateWalkEnd`. Sight from the end is the one sight test (`sightReach.Reaches`, as `SeesWithin` asks it).
- `ValidateWalkEnd`'s law is read once for every cell through `walkEndsRefused`, so `Approaches` asks the walker's view once and holds no second copy of the rule.
- `Approaches` is listed among the answers in [AGENTS.md](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/AGENTS.md).
- Tests: [approach_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/approach_test.go) and [approach_bench_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/59b274fb/rulebooks/dnd5e/encounter/approach_bench_test.go).

## Decided

- A budget is one bounded flood, however many targets are asked. Each target then reads only the cells within reach of it, cheapest first, ties by X then Y.
- No budget is one goal-directed search per target (`searchToward`), not one flood. The unbounded walk flood is the whole floor, about 48 ms on the 10k keep, which would put a free-roam Afford past its 100 ms target. The search returns the flood's own cell and path by its rpg-project#762 promise. `TestApproaches_CastleHoldsTheOracle` holds it to a brute-force unbounded flood on the castle.
- A target in reach must also be seen from the end cell. A cell across a wall from the target is within reach and sees nothing, and Attack's candidates are sight holdings.
- The fold is read live, as the step reads it. A body the mover has not seen still shapes the answer, because the walk would meet it and stop.

## Found

- Benchmarks on the 10k keep with its 99-monster garrison, asked of every monster at five feet:

| read | ms/op |
|---|---|
| fight, 6-step budget, one flood | 1.05 |
| free roam, one target, one search | 6.2 |
| free roam, all 99 targets, 99 searches | 156 |
| free roam, one unbounded flood (first build) | 48 |

- A target no walk reaches costs its search every cell the mover can walk, as the flood would. Afford asks only the candidates its attacker sees, so the all-garrison row is not a read Afford makes.
- Member seats in a `MemberInput` are authored offset pairs, while door edges and every reported cell are axial. The first fixture placed members by `cellAt` and built the wrong scene.
- Mutants killed: a walk that opens doors (the search and the flood each), no sight check, no budget, no cheapest-first sort, a pass-through end accepted, a search reach one cell too wide.

— cross-team agent, on behalf of KirkDiggler
