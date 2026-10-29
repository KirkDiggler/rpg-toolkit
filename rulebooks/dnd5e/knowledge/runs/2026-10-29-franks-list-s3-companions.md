# Run — 2026-10-29 — Frank's list S3, companions

Slice [rpg-project#717](https://github.com/KirkDiggler/rpg-project/issues/717) of story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), under lane [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722) ruling D2. Base: rpg-toolkit main at [`61bb0381`](https://github.com/KirkDiggler/rpg-toolkit/commit/61bb0381). PRs: [rpg-toolkit#2156](https://github.com/KirkDiggler/rpg-toolkit/pull/2156) (`mind/behavior`) and [rpg-toolkit#2157](https://github.com/KirkDiggler/rpg-toolkit/pull/2157) (`encounter`, on #2156's pseudo-version).

## Shipped

- `mind/behavior` reads a selector `ally`: the nearest allied member held in sight, else in memory. It is legal on `toward`, `dash` and `away`, and refused on `attack`. `Facts.SeenAllies` and `Facts.RememberedAllies` carry who is held — [`SelectorAlly`](../../../../mind/behavior/table.go), [rpg-toolkit#2156](https://github.com/KirkDiggler/rpg-toolkit/pull/2156).
- A disposition's `until` is one step or a list of steps. Each step is one predicate and an optional `then:`, the stance it turns the pair to, and the first step that holds wins. A step with no `then` turns the pair to the other of hostile and neutral, so every existing file means what it meant. An `until` on an allied pair is refused only when a step names no `then` — [`UntilStep`](../../encounter/field.go), [`validateSteps`](../../encounter/disposition.go), [`UntilSpec`](../../encounter/dungeonspec/predicate.go), [`grammar.steps`](../../encounter/dungeonspec/factions.go), ruled 2026-10-29 on [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- The graph declares a list's fact steps last step first, so the first step's settle is projected last and wins whichever fact was learned first — [`buildWorld`](../../encounter/world.go).
- A one-step `until` persists as `until` and `then`, byte for byte the shape saved runs have. A list persists as `steps` — [`DispositionData`](../../encounter/data.go).
- A pair whose `until` turns it allied settles to the allied edge, through its mind or publicly. The load trust boundary accepts a `settled:allied` fact only for such a pair — [`field.settledStancesOf`](../../encounter/world.go).
- The driver resolves `ally` off the view: the nearest member of a faction allied with the creature's own, sighted and standing first, then remembered — [`nearestAllied`](../../encounter/tabledriver.go), [`Encounter.alliedAcross`](../../encounter/world.go).
- An allied sighting has no reach, and its route ends beside it rather than at weapon range. A follower already beside its ally stands and says "already beside its ally" — [`SeenMember.Allied`](../../encounter/turndriver.go), [`buildMonsterView`](../../encounter/clocks.go).
- The walk site `ally-walk` is cut from the castle through the gate walk. Its scenario test proves join, follow, and a fight beside the party that never strikes the hero — [`ally_walk_test.go`](../../encounter/castletest/ally_walk_test.go), [`ally-walk.yaml`](../../encounter/castletest/testdata/ally-walk.yaml).

- The castle carries both turns. The garrison and party are neutral, `until: [{ fact: guards-turned, then: allied }, { fact: party-recognized, then: hostile }]`, and the captain's march is now `toward: ally`. The castle routes prove recognition alone turns hostile, the turn alone turns allied, and the captain then follows the hero into the keep and strikes loyalists, never the hero. They also prove that recognized-then-turned and turned-then-recognized both end allied — [castle dispositions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml), [`castle_king_keep_test.go`](../../encounter/castletest/castle_king_keep_test.go), [`castle_king_companion_test.go`](../../encounter/castletest/castle_king_companion_test.go).
- A castle authored with the march in place of the follow row keeps its captain behind: he is the party's side, never follows, and marches on the keep — [`TestMadKingACaptainWithNoFollowRowStaysBehind`](../../encounter/castletest/castle_king_companion_test.go).

## Decided

- `ally` excludes the creature's own faction. A faction is allied with itself by law, so a turned captain following the nearest ally would follow the next guard, not the party. D2 did not say which members count — [`Encounter.alliedAcross`](../../encounter/world.go), [D2 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- `then:` lives inside each step, one stance word: `until: { fact: x, then: allied }` alone, or a list of the same. The sibling `then:` beside `until:` that the first commit carried is gone, so there is one spelling — [`DispositionSpec.Until`](../../encounter/dungeonspec/spec.go).
- A list of more than one step names facts only. Public turns, a round, a fall or another pair's stance, are ordered by stance and after every fact, so in a list they could not keep their place. One step of any form stays legal — [`untilListOnFacts`](../../encounter/disposition.go).
- A step that turns the pair to its own declared stance is refused by name, in the run and in the file — [`validateSteps`](../../encounter/disposition.go).
- The castle's captain's march is replaced by his follow row, not joined by it. Two rows on the same `when` would split his roll between following and marching — [castle tables](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- The public allied settle sits between neutral and hostile in projection order, so a later public turn to hostile overrules an alliance — [`settledStances`](../../encounter/world.go).
- A companion stays a monster placement driven by its own table. It is not moved into the party roster or the party's turn order — [rpg-project#717](https://github.com/KirkDiggler/rpg-project/issues/717).

## Found

- The castle's garrison and party pair already had an `until` on `party-recognized`, which turns it hostile. One pair held one `until`, and the castle wanted neutral to hostile on recognition and neutral to allied on the captain's turn. Those are two facts from the same state, not talk-then-fight, which made this the first real use case for more than one `until` on a pair. The operator ruled the list on it — [D4 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- A follower one step behind a hero who walks into a fight does not join it, because it sees no foe from the doorway. It keeps following on the world's rounds between turns, and joins once it comes through and sees a loyalist itself. A hero who stops at the door therefore fights alone, so the castle route presses on two cells a turn — [`pressIntoTheKeep`](../../encounter/castletest/castle_king_keep_test.go).
- rpg-api's `content/the-mad-king.yaml` has to mirror the castle's new `until` list and the captain's follow row before the castle is walked — [rpg-api content](https://github.com/KirkDiggler/rpg-api/blob/dev/content/the-mad-king.yaml).
- A member who joins a fight during another's driven turn holds the active slot undriven until somebody ends it. The site's route ends whoever is active, as the castle's attacker route does — [`ally_walk_test.go`](../../encounter/castletest/ally_walk_test.go).
- `away: ally` and `away: enemy` anchor a `Routed` walk on the member's live cell at execution, not the believed one — [`Routed.Anchor`](../../encounter/turndriver.go).
- The castle host's positions use two frames. `MemberInput.Position` takes an offset cell and `Members` and `Route` answer absolute ones, so a walk target needs `castleCell` — [`castle_king_test.go`](../../encounter/castletest/castle_king_test.go).
- The content goldens marshal `DispositionInput` raw, so every committed picture gained an empty `Then` — [`contentgolden_test.go`](../../encounter/dungeonspec/contentgolden_test.go).
- The party roster, the turn order showing the companion on the party's side, and the recap's "who came with you" are session, api and web work — [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714).
- The lint pin is v2.15.0 since [rpg-toolkit#2148](https://github.com/KirkDiggler/rpg-toolkit/pull/2148), so no go1.25 run was needed — [`.golangci-version`](../../../../.golangci-version).

## Mistaken

- The first `then:` sat beside `until:`, which could spell one step only. The operator's list made it a key of each step, and the sibling was removed before anything pinned it — [rpg-toolkit#2157](https://github.com/KirkDiggler/rpg-toolkit/pull/2157).

- The first site put the thugs in the gatehouse's north corners. Lines through the open south door reached both, and the fight formed before anyone spoke. A brute-force sight check found the two south corners hidden from the whole outside route — [`ally-walk.yaml`](../../encounter/castletest/testdata/ally-walk.yaml).
- The first scenario passed walk targets as offset cells to `Route`, which takes absolute ones, so the hero barely moved and the follow assertion passed on marking time alone — [`ally_walk_test.go`](../../encounter/castletest/ally_walk_test.go).
