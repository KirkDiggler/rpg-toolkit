# Run — 2026-10-29 — Frank's list S3, companions

Slice [rpg-project#717](https://github.com/KirkDiggler/rpg-project/issues/717) of story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), under lane [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722) ruling D2. Base: rpg-toolkit main at [`61bb0381`](https://github.com/KirkDiggler/rpg-toolkit/commit/61bb0381). PRs: [rpg-toolkit#2156](https://github.com/KirkDiggler/rpg-toolkit/pull/2156) (`mind/behavior`) and [rpg-toolkit#2157](https://github.com/KirkDiggler/rpg-toolkit/pull/2157) (`encounter`, on #2156's pseudo-version).

## Shipped

- `mind/behavior` reads a selector `ally`: the nearest allied member held in sight, else in memory. It is legal on `toward`, `dash` and `away`, and refused on `attack`. `Facts.SeenAllies` and `Facts.RememberedAllies` carry who is held — [`SelectorAlly`](../../../../mind/behavior/table.go), [rpg-toolkit#2156](https://github.com/KirkDiggler/rpg-toolkit/pull/2156).
- A disposition's `until` may name the stance it turns to with `then:`. Omitted, it turns to the other of hostile and neutral as before. An `until` on an allied pair is refused only when it names no `then` — [`validateThen`](../../encounter/disposition.go), [`grammar.then`](../../encounter/dungeonspec/factions.go).
- A pair whose `until` turns it allied settles to the allied edge, through its mind or publicly. The load trust boundary accepts a `settled:allied` fact only for such a pair — [`field.settledStancesOf`](../../encounter/world.go).
- The driver resolves `ally` off the view: the nearest member of a faction allied with the creature's own, sighted and standing first, then remembered — [`nearestAllied`](../../encounter/tabledriver.go), [`Encounter.alliedAcross`](../../encounter/world.go).
- An allied sighting has no reach, and its route ends beside it rather than at weapon range. A follower already beside its ally stands and says "already beside its ally" — [`SeenMember.Allied`](../../encounter/turndriver.go), [`buildMonsterView`](../../encounter/clocks.go).
- The walk site `ally-walk` is cut from the castle through the gate walk. Its scenario test proves join, follow, and a fight beside the party that never strikes the hero — [`ally_walk_test.go`](../../encounter/castletest/ally_walk_test.go), [`ally-walk.yaml`](../../encounter/castletest/testdata/ally-walk.yaml).

## Decided

- `ally` excludes the creature's own faction. A faction is allied with itself by law, so a turned captain following the nearest ally would follow the next guard, not the party. D2 did not say which members count — [`Encounter.alliedAcross`](../../encounter/world.go), [D2 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- The dialect key is `then:` beside `until:`, one stance word. No list, because D4 deferred one — [`DispositionSpec.Then`](../../encounter/dungeonspec/spec.go).
- The public allied settle sits between neutral and hostile in projection order, so a later public turn to hostile overrules an alliance — [`settledStances`](../../encounter/world.go).
- A companion stays a monster placement driven by its own table. It is not moved into the party roster or the party's turn order — [rpg-project#717](https://github.com/KirkDiggler/rpg-project/issues/717).

## Found

- The castle's garrison and party pair already has an `until` on `party-recognized`, which turns it hostile. One pair holds one `until`, so the same pair cannot also turn allied on `guards-turned` without the list D4 deferred — [castle dispositions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml), [D4 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- A member who joins a fight during another's driven turn holds the active slot undriven until somebody ends it. The site's route ends whoever is active, as the castle's attacker route does — [`ally_walk_test.go`](../../encounter/castletest/ally_walk_test.go).
- `away: ally` and `away: enemy` anchor a `Routed` walk on the member's live cell at execution, not the believed one — [`Routed.Anchor`](../../encounter/turndriver.go).
- The castle host's positions use two frames. `MemberInput.Position` takes an offset cell and `Members` and `Route` answer absolute ones, so a walk target needs `castleCell` — [`castle_king_test.go`](../../encounter/castletest/castle_king_test.go).
- The content goldens marshal `DispositionInput` raw, so every committed picture gained an empty `Then` — [`contentgolden_test.go`](../../encounter/dungeonspec/contentgolden_test.go).
- The party roster, the turn order showing the companion on the party's side, and the recap's "who came with you" are session, api and web work — [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714).
- The lint pin is v2.15.0 since [rpg-toolkit#2148](https://github.com/KirkDiggler/rpg-toolkit/pull/2148), so no go1.25 run was needed — [`.golangci-version`](../../../../.golangci-version).

## Mistaken

- The first site put the thugs in the gatehouse's north corners. Lines through the open south door reached both, and the fight formed before anyone spoke. A brute-force sight check found the two south corners hidden from the whole outside route — [`ally-walk.yaml`](../../encounter/castletest/testdata/ally-walk.yaml).
- The first scenario passed walk targets as offset cells to `Route`, which takes absolute ones, so the hero barely moved and the follow assertion passed on marking time alone — [`ally_walk_test.go`](../../encounter/castletest/ally_walk_test.go).
