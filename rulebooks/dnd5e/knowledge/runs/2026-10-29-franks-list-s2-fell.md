# Run — 2026-10-29 — Frank's list S2, the fell deed

Slice [rpg-project#716](https://github.com/KirkDiggler/rpg-project/issues/716) of story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), under lane [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722) rulings D1 and D6. Base: rpg-toolkit main at [`dd2ccc8b`](https://github.com/KirkDiggler/rpg-toolkit/commit/dd2ccc8b).

## Shipped

- `fell` joins the `when` deed words, a deed condition may carry `who` (legal only on the words in `WhoDeeds`, today `fell`), and `HeldDeed` carries the deed's `Target` that `who` is compared against — [rpg-toolkit#2149](https://github.com/KirkDiggler/rpg-toolkit/pull/2149).
- A member going down lands `DeedFell` on every member whose sight reaches it, beside the down beat that tells it, naming who dropped it and who fell — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
- The facts projection carries the target, and `WhenData` persists `who` — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
- The dialect reads `fell: { who, within, on/as }`, refuses `who` on any other deed by that deed's name, and seals it to a monster placed in the file — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
- The castle's two knights carry `{ when: { fell: { who: king, within: 3, on: ally } }, away: enemy, weight: 3 }`, and a castle route proves the knight who saw the king fall has the row and the one who did not does not — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).

## Decided

- The shape is `When.Who` and `HeldDeed.Target`. `Target` is the name `deed.Deed` already gives the field it is projected from, so the fall needs no new payload and every verb's target rides along — [D1 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- The fall's actor is the last attack the noticing verb landed on the fallen, held in a field that lives inside one call and is never persisted. A fall no attack in that verb explains is filed as the fallen's own, because the actor keys the deed's subject and `stage.Land` refuses an empty one — [rpg-project#716](https://github.com/KirkDiggler/rpg-project/issues/716).
- The witnesses are the members whose sight reaches the fallen's cell, plus the one who dropped it. The fallen witnesses its own fall, the way a target knows it was struck — [rpg-project#716](https://github.com/KirkDiggler/rpg-project/issues/716).
- The castle row carries `on: ally`. Without a scope a deed condition reads deeds against the creature itself, and that row could never hold for the king's fall — [rpg-project#716](https://github.com/KirkDiggler/rpg-project/issues/716).
- An empty `who:` is refused rather than read as every fall, since omitting `who` already means that — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
- The castle route opens without `king-slain`, and that choice is stated in the route, not in the file — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).

## Found

- The castle's `king-slain` ending closes the run the moment the king is down, so in the castle as authored no knight ever picks again and the `fell` row is never rolled. The walk needs a call on the ending: move it, keep it and add a reserve wave (D6), or accept the row only in other files — [castle endings](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- A witness holds one deed per actor and verb ([`deed.Subject`](../../../../mind/behavior/deed/deed.go)). When the same hero drops the king and then a loyalist, every witness's fall by that hero now names the loyalist, and `fell: { who: king }` stops holding inside its span. Keying `fell` by its target too is a change to the deeds channel's filing law in `mind/behavior`, and is the lane's to rule — [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- `away: actor` on a `fell` row resolves through the table driver's freshest-deed read, which does not apply `who`. With two falls in a knight's memory, `actor` can name the wrong dropper. The driver is the doors builder's file and was not touched — [`tabledriver.go`](../../encounter/tabledriver.go).
- A fall nobody in the verb caused names the fallen as its actor, so `away: actor` on such a row walks away from the body — [`deed.go`](../../encounter/deed.go).
- The World Builder dialect keeps props out of its cast, so `who: altar` is refused as not a placement, the sentence `down: altar` already earns, rather than as a prop — [`grammar.go`](../../encounter/dungeonspec/grammar.go).
- The web form offering `fell` and `who` (`answerVocabulary.ts`) is the third PR of this slice and is not started — [rpg-project#716](https://github.com/KirkDiggler/rpg-project/issues/716).
- The repo's `.githooks/pre-commit` is not installed in this clone, so its checks were run by hand — [run-tests how-to](../../../../docs/how-to/run-tests.md).
- The local `golangci-lint` is v2.15.0, not the pinned v2.3.1, and reports goconst findings main does not have in CI. The pinned binary panics under Go 1.27 and runs clean under go1.25.14, CI's toolchain — [`.golangci-version`](../../../../.golangci-version).

## Mistaken

- The first castle route ended the hero's turn and then the fighter's directly and was refused as not the active member, because the knights fight in two bubbles. It now ends each player's turn as it comes up — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
- The first encounter test asserted the fall's place in offset coordinates, while a deed's place is the board's absolute cell — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
- The first castle seal test expected the v2 dialect's prop sentence, which the World Builder dialect does not produce — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151).
