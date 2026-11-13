# 2026-11-03 · A cast's shoves land inside its landing · Session

processed: 2026-11-12T165555Z

Bug [rpg-project#831](https://github.com/KirkDiggler/rpg-project/issues/831), law [rpg-project#785](https://github.com/KirkDiggler/rpg-project/issues/785) R3, seen on the S1 walk of [rpg-project#825](https://github.com/KirkDiggler/rpg-project/issues/825). Session PR [rpg-toolkit#2247](https://github.com/KirkDiggler/rpg-toolkit/pull/2247), on encounter [rpg-toolkit#2248](https://github.com/KirkDiggler/rpg-toolkit/pull/2248).

## Shipped

- [`landingPushes`](https://github.com/KirkDiggler/rpg-toolkit/blob/48a91b1e/rulebooks/dnd5e/session/castmove.go#L262) splits a cast's routed pushes once, before the landing, at [`cast.go:522`](https://github.com/KirkDiggler/rpg-toolkit/blob/48a91b1e/rulebooks/dnd5e/session/cast.go#L522). Shoves go to `RecordCast` at [`cast.go:541`](https://github.com/KirkDiggler/rpg-toolkit/blob/48a91b1e/rulebooks/dnd5e/session/cast.go#L541). Flees are walked in Continue through `walkCastPushes`, as before.
- [`displaces`](https://github.com/KirkDiggler/rpg-toolkit/blob/48a91b1e/rulebooks/dnd5e/session/castmove.go#L286) is the one place this package reads `Provokes` as a kind of move.
- `go.mod` pins encounter as a pseudo-version of 3a325117, the last #2248 commit inside the encounter module, with no replace.
- The test [`TestCastleFreeRoamThunderwaveShovesBeforeTheFight`](https://github.com/KirkDiggler/rpg-toolkit/blob/48a91b1e/rulebooks/dnd5e/session/castle_free_roam_test.go#L296) asserts cast before shove, shove before the fight forms, shove before the first turn ends, and a pushed guard's first step from its pushed cell.

## Decided

- A shove is handed to the encounter on every clock, not only on the world clock. The push is part of the cast landing wherever the cast lands.
- A push with no route is dropped from both lists. The cast's own moved result already tells a push of zero cells.

## Found

- The flee half is still open: a Dissonant Whispers or Command flee from free roam walks after the fight its cast formed. It is [rpg-project#832](https://github.com/KirkDiggler/rpg-project/issues/832).
- `displaces` becomes `moveKindFor(provokes) == encounter.MoveDisplaced` at the merge with [rpg-toolkit#2246](https://github.com/KirkDiggler/rpg-toolkit/pull/2246). `moveKindFor` returns `encounter.MoveKind`, which the pinned encounter does not have until [rpg-toolkit#2245](https://github.com/KirkDiggler/rpg-toolkit/pull/2245). That merge also conflicts in `castmove.go`, where both PRs add a function after `walkCastPushes`; keep both. It conflicts in `go.mod` too; re-pin to an encounter that carries both #2245 and #2248.
- Under the old order, a fight whose first turns dropped the caster closed the story. The push walk was then refused, `encounter closed`, and the whole cast scope was dropped.

## Mistaken

- The first commit walked the shoves before `RecordCast` on the world clock. That put the moved beats ahead of the cast beat, which tells the wrong story in the record. The encounter half replaced it.
- The Record closure reassigned the captured `pushes`, which Continue then read. That was safe only because `land` calls Record once. The split now happens before `land`.
- Two heads failed CI's dirty-tree check: once on an untidied `go.sum`, once for want of this record.

— cross-team agent, on behalf of KirkDiggler
