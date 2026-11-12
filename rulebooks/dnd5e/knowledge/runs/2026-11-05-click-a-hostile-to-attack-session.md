# 2026-11-05 · Click a hostile to attack, D4 · Session

processed: 2026-11-12T165555Z

Story [rpg-project#872](https://github.com/KirkDiggler/rpg-project/issues/872), decision D4. Built on session main 6d8a753a, pinning encounter at [56dffeaa](https://github.com/KirkDiggler/rpg-toolkit/commit/56dffeaabbc934e8dc6ea1dff466ca30a8cb505b) (`v0.178.1-0.20261106010014-56dffeaabbc9`), the head of #2285 after its review fix, in commit [4e8394f2](https://github.com/KirkDiggler/rpg-toolkit/commit/4e8394f2480d72d6681d2ab3c641675848777af0). Wire field: [rpg-api-protos#420](https://github.com/KirkDiggler/rpg-api-protos/pull/420), `TargetCandidate.approach`.

## Shipped

- [`TargetCandidate.Approach`](https://github.com/KirkDiggler/rpg-toolkit/blob/4e8394f2/rulebooks/dnd5e/session/types.go) (`approach`, omitted when nil) is the cell the attacker's own walk this turn ends on within the weapon's reach and in sight of that candidate. It is set only when the candidate is out of reach and such a walk exists.
- [`approachCandidates`](https://github.com/KirkDiggler/rpg-toolkit/blob/4e8394f2/rulebooks/dnd5e/session/reach.go) asks `encounter.Approaches` once per compiled Attack, after the participation filter. The range is the weapon's `MaxRangeFeet`. On the turn clock the budget is the readied sheet's movement left, five feet a cell, and only for the active member. On the world clock the walk is unbounded.
- Tests: [castle_approach_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/4e8394f2/rulebooks/dnd5e/session/castle_approach_test.go), a wire-name pin in [wirecontract_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/4e8394f2/rulebooks/dnd5e/session/wirecontract_test.go), and [castle_approach_bench_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/4e8394f2/rulebooks/dnd5e/session/castle_approach_bench_test.go).

## Decided

- The law is pinned end to end on the castle: Afford names a cell, a Move along the shortest walk there completes, Afford then offers the Attack with no approach, and the Attack lands. It runs for the captain and both gate guards in free roam, and for gate-1 in a fight with 15 feet left.
- With 10 feet left in a fight, a guard whose walk is longer than two cells gets no cell, and any cell named is inside the budget.
- A glaive stops two cells off gate-1, not beside him, and the swing lands from there.
- Approach is not selector material. Attack execution regenerates the same offer and pays the same read.
- `openTheGatehouseAs` takes alice's sheet so the glaive case runs on the same castle.

## Found

- Afford before and after, 20 iterations, three runs each:

| read | before ms/op | after ms/op |
|---|---|---|
| keep 10k at launch, a fight, 4 candidates, 2 answered | 64.9 | 67.7 |
| castle gatehouse, free roam, 3 guards out of reach | 17.9 | 19.5 |

- The keep's party starts in a fight, so its benchmark measures the bounded flood. With the call disabled the keep measures 64.5 ms, so the call is the whole difference.
- Mutants killed: the world clock's budget used in a fight, movement feet read as cells, a sword's reach used for a glaive.

— cross-team agent, on behalf of KirkDiggler
