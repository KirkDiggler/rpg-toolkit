# Run — 2026-10-29 — Frank's list S4, the survivors

processed: 2026-10-30T004357Z

Slice [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718) of story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), under lane [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722) ruling D4. Base: rpg-toolkit main at [`61bb0381`](https://github.com/KirkDiggler/rpg-toolkit/commit/61bb0381), with main merged in at [`78a28eff`](https://github.com/KirkDiggler/rpg-toolkit/commit/78a28eff), [`a1e058e8`](https://github.com/KirkDiggler/rpg-toolkit/commit/a1e058e8) and [`93b323e6`](https://github.com/KirkDiggler/rpg-toolkit/commit/93b323e6). PR: [rpg-toolkit#2159](https://github.com/KirkDiggler/rpg-toolkit/pull/2159).

## Shipped

- A faction may carry `options`. A member that writes its own keeps them wholesale, and a member that writes none speaks its faction's, in both dialects — [`inherited.optionsFor`](../../encounter/dungeonspec/compile.go), [rpg-toolkit#2159](https://github.com/KirkDiggler/rpg-toolkit/pull/2159).
- The one option grammar judges a faction's lines at `factions[i].options[j]`. Their facts count toward what a file sets, their `use:` rows resolve, and a null `options:` is refused — [`grammar.factionOptions`](../../encounter/dungeonspec/grammar.go), [`bundles.go`](../../encounter/dungeonspec/bundles.go).
- A down creature answers nothing. `Speak` refuses a line said to it with `ErrMemberDown` before anything is written, and `OptionsFor` offers it no lines — [`option.go`](../../encounter/option.go), [`errors.go`](../../encounter/errors.go).
- The castle's loyalists carry `yield` and `refuse`, and `loyalists`↔`party` is hostile `until: { down: king }` — [castle factions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- Scenes prove the fall dissolves the fight, every survivor offers the faction's lines, a yield resolves with no swing, a yield's fact folds a second disposition's `until`, and a refusal then a blow either way turns the pair hostile again — [`survivors_test.go`](../../encounter/survivors_test.go), [`castle_king_survivors_test.go`](../../encounter/castletest/castle_king_survivors_test.go).
- The walk site `survivors-walk` is cut from the gate walk's frame. It holds a band of three with a two-hit-point leader and the faction's two lines, untracked in the franks-list-s1 api worktree and the castle stack's content — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).

## Decided

- The faction's lines layer by the whole key, never merged by id. That is `on:`'s rule, and a member's own list is visible where it is written — [`FactionSpec.Options`](../../encounter/dungeonspec/spec.go).
- An empty `options: []` is refused on a creature, a prop and a faction, the sentence an empty verdict earns. `options` is a layered key, so absent means the layer below speaks, and an empty list read as absent would hand a member its faction's lines — [`grammar.options`](../../encounter/dungeonspec/grammar.go), [review on rpg-toolkit#2159](https://github.com/KirkDiggler/rpg-toolkit/pull/2159#pullrequestreview-5705645732).
- No `until` list. A refusing survivor who swings, or is struck, turns the pair hostile by the provocation law — [D4 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- A down member is refused at the encounter's door with its own sentinel, not `ErrNotDown`, because the two mean opposite things. The offer agrees with the door, so a panel never shows a line the door would refuse — [`option.go`](../../encounter/option.go).
- The castle's `yield` is ungated. `requires` reads a fact or a holding, and nothing the king's fall teaches is a fact — [castle factions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- The castle route first opened without `king-slain`, through S2's `without` host field, while that ending closed the run at the fall. Once [rpg-toolkit#2155](https://github.com/KirkDiggler/rpg-toolkit/pull/2155) was merged in, the route plays on the file's own endings and asserts the fall ends nothing — [`castle_king_survivors_test.go`](../../encounter/castletest/castle_king_survivors_test.go).
- The walk site's band has a follower for its mind, so a fact `until` on the band could still settle after the leader falls — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).

## Found

- An option's `requires` cannot read a `down`. It takes `{ fact }` or `{ holds }` only, so a line meant for after a fall cannot be gated on the fall — [`grammar.optionRequires`](../../encounter/dungeonspec/grammar.go).
- Before this slice, a line said to a down creature was answered. The fallen leader said "We yield.", learned `band-yielded` and held the persuade deed — [`option.go`](../../encounter/option.go).
- A down member is still a witness. `audienceOf` listed the fallen leader among those who saw the hero, so a body still lands deeds and learns facts from a line said near it — [`deed.go`](../../encounter/deed.go).
- A fact `until` settles through the faction's mind, and the castle's loyalists' mind is the king. After his fall no loyalist pair can turn on a fact, so `loyalists-yielded` folds nothing for the court — [castle factions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- Before [rpg-toolkit#2155](https://github.com/KirkDiggler/rpg-toolkit/pull/2155), the castle's `king-slain` ended the run at the fall, so `Speak` after it returned `ErrClosed`. With #2155 merged in at [`93b323e6`](https://github.com/KirkDiggler/rpg-toolkit/commit/93b323e6), every ending waits for the front door and the survivors route needs no host override — [`castle_king_survivors_test.go`](../../encounter/castletest/castle_king_survivors_test.go).
- This castle fold broke S2's `TestMadKingAKnightWhoSawTheKingFallHasTheRow` when main was merged in. The court stands down at the fall, both knights' fights dissolve, and no knight picks again, so the `fell` row is never rolled. Removing the `until` made it pass. The route now strikes keep-1 after the fall, the pair turns hostile by provocation, and the test's four claims hold unchanged. The open question is for the lane. S2's `fell` rows are testimony, so a knight who did not see the fall fights on, while `until: { down }` is world truth that stands the whole court down at once — [`castle_king_fell_test.go`](../../encounter/castletest/castle_king_fell_test.go), [D12 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- `session` has no arm for `encounter.ErrMemberDown`. No path reaches it, because `session` asks `OptionsFor` before it charges, and the refusal there is `ErrNoOption`. `refuseIfDown` on the target would give the exact sentence — [`session/social.go`](../../session/social.go).
- The studio's Factions panel has no Options editor, and the Read back does not say a member's lines are its faction's — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).
- The walk site serves only once the api worktree pins this head. Its current pin is `7fc83d10`, which refuses `options` on a faction — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).
- rpg-api's `content/the-mad-king.yaml` has to mirror the loyalists' `options` block and the `until` before the castle walk — [rpg-api content](https://github.com/KirkDiggler/rpg-api/blob/dev/content/the-mad-king.yaml).
- The lint pin is v2.15.0 since [rpg-toolkit#2148](https://github.com/KirkDiggler/rpg-toolkit/pull/2148), so the local binary matches CI and no go1.25 run was needed — [`.golangci-version`](../../../../.golangci-version).

## Mistaken

- The first fact-pattern test wrote `/yielded/` as if a pattern matched part of a name. A pattern matches the whole name, so it now reads `/band-.*/` — [`faction_options_test.go`](../../encounter/dungeonspec/faction_options_test.go).
- The first survivors scene put the watch in every yard. Band and watch kept fighting after the leader fell, and "the fight dissolves" failed. The watch now appears only in the scene about the second disposition — [`survivors_test.go`](../../encounter/survivors_test.go).
- The first cut read `options: []` as absent, so a member that wrote an empty list spoke its faction's lines. Found in review and refused — [review on rpg-toolkit#2159](https://github.com/KirkDiggler/rpg-toolkit/pull/2159#pullrequestreview-5705645732).
- The first down-refusal test counted only facts, so a refusal after the beat or the deed would have passed it. It now counts every member's story and checks no persuade deed is held — [`survivors_test.go`](../../encounter/survivors_test.go).
- The first trial merge of the other builders' branches committed conflicted files into a scratch tree. Only #2155's merge was kept for the check, and the scratch tree was removed — [rpg-toolkit#2155](https://github.com/KirkDiggler/rpg-toolkit/pull/2155).
