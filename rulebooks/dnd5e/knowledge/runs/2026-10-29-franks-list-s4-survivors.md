# Run — 2026-10-29 — Frank's list S4, the survivors

Slice [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718) of story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), under lane [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722) ruling D4. Base: rpg-toolkit main at [`61bb0381`](https://github.com/KirkDiggler/rpg-toolkit/commit/61bb0381). PR: [rpg-toolkit#2159](https://github.com/KirkDiggler/rpg-toolkit/pull/2159).

## Shipped

- A faction may carry `options`. A member that writes its own keeps them wholesale, and a member that writes none speaks its faction's, in both dialects — [`inherited.optionsFor`](../../encounter/dungeonspec/compile.go), [rpg-toolkit#2159](https://github.com/KirkDiggler/rpg-toolkit/pull/2159).
- The one option grammar judges a faction's lines at `factions[i].options[j]`. Their facts count toward what a file sets, their `use:` rows resolve, and a null `options:` is refused — [`grammar.factionOptions`](../../encounter/dungeonspec/grammar.go), [`bundles.go`](../../encounter/dungeonspec/bundles.go).
- A down creature answers nothing. `Speak` refuses a line said to it with `ErrMemberDown` before anything is written, and `OptionsFor` offers it no lines — [`option.go`](../../encounter/option.go), [`errors.go`](../../encounter/errors.go).
- The castle's loyalists carry `yield` and `refuse`, and `loyalists`↔`party` is hostile `until: { down: king }` — [castle factions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- Scenes prove the fall dissolves the fight, every survivor offers the faction's lines, a yield resolves with no swing, a yield's fact folds a second disposition's `until`, and a refusal then a blow either way turns the pair hostile again — [`survivors_test.go`](../../encounter/survivors_test.go), [`castle_king_survivors_test.go`](../../encounter/castletest/castle_king_survivors_test.go).
- The walk site `survivors-walk` is cut from the gate walk's frame. It holds a band of three with a two-hit-point leader and the faction's two lines, untracked in the franks-list-s1 api worktree and the castle stack's content — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).

## Decided

- The faction's lines layer by the whole key, never merged by id. That is `on:`'s rule, and a member's own list is visible where it is written — [`FactionSpec.Options`](../../encounter/dungeonspec/spec.go).
- No `until` list. A refusing survivor who swings, or is struck, turns the pair hostile by the provocation law — [D4 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- A down member is refused at the encounter's door with its own sentinel, not `ErrNotDown`, because the two mean opposite things. The offer agrees with the door, so a panel never shows a line the door would refuse — [`option.go`](../../encounter/option.go).
- The castle's `yield` is ungated. `requires` reads a fact or a holding, and nothing the king's fall teaches is a fact — [castle factions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- The castle route opens without `king-slain` by id, and the route says so. The content under test is the court's lines and `until`, and neither reads an ending — [`castle_king_survivors_test.go`](../../encounter/castletest/castle_king_survivors_test.go).
- The walk site's band has a follower for its mind, so a fact `until` on the band could still settle after the leader falls — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).

## Found

- An option's `requires` cannot read a `down`. It takes `{ fact }` or `{ holds }` only, so a line meant for after a fall cannot be gated on the fall — [`grammar.optionRequires`](../../encounter/dungeonspec/grammar.go).
- Before this slice, a line said to a down creature was answered. The fallen leader said "We yield.", learned `band-yielded` and held the persuade deed — [`option.go`](../../encounter/option.go).
- A down member is still a witness. `audienceOf` listed the fallen leader among those who saw the hero, so a body still lands deeds and learns facts from a line said near it — [`deed.go`](../../encounter/deed.go).
- A fact `until` settles through the faction's mind, and the castle's loyalists' mind is the king. After his fall no loyalist pair can turn on a fact, so `loyalists-yielded` folds nothing for the court — [castle factions](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- On main the castle's `king-slain` ends the run at the fall, so `Speak` after it returns `ErrClosed`. With [rpg-toolkit#2155](https://github.com/KirkDiggler/rpg-toolkit/pull/2155) merged into this branch, the castle route passes with the file's real endings — [`castle_king_survivors_test.go`](../../encounter/castletest/castle_king_survivors_test.go).
- This castle fold breaks S2's `TestMadKingAKnightWhoSawTheKingFallHasTheRow` once the two meet. The court stands down at the fall, the fight dissolves, and no knight picks again, so the `fell` row is never rolled. Removing the `until` makes the test pass again. S2's `fell` rows are testimony-driven, so a knight who did not see the fall fights on. `until: { down }` is world truth and stands the whole faction down at once — [rpg-toolkit#2151](https://github.com/KirkDiggler/rpg-toolkit/pull/2151), [D6 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722).
- `session` has no arm for `encounter.ErrMemberDown`. No path reaches it, because `session` asks `OptionsFor` before it charges, and the refusal there is `ErrNoOption`. `refuseIfDown` on the target would give the exact sentence — [`session/social.go`](../../session/social.go).
- The studio's Factions panel has no Options editor, and the Read back does not say a member's lines are its faction's — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).
- The walk site serves only once the api worktree pins this head. Its current pin is `7fc83d10`, which refuses `options` on a faction — [rpg-project#718](https://github.com/KirkDiggler/rpg-project/issues/718).
- rpg-api's `content/the-mad-king.yaml` has to mirror the loyalists' `options` block and the `until` before the castle walk — [rpg-api content](https://github.com/KirkDiggler/rpg-api/blob/dev/content/the-mad-king.yaml).
- The lint pin is v2.15.0 since [rpg-toolkit#2148](https://github.com/KirkDiggler/rpg-toolkit/pull/2148), so the local binary matches CI and no go1.25 run was needed — [`.golangci-version`](../../../../.golangci-version).

## Mistaken

- The first fact-pattern test wrote `/yielded/` as if a pattern matched part of a name. A pattern matches the whole name, so it now reads `/band-.*/` — [`faction_options_test.go`](../../encounter/dungeonspec/faction_options_test.go).
- The first survivors scene put the watch in every yard. Band and watch kept fighting after the leader fell, and "the fight dissolves" failed. The watch now appears only in the scene about the second disposition — [`survivors_test.go`](../../encounter/survivors_test.go).
- The first trial merge of the other builders' branches committed conflicted files into a scratch tree. Only #2155's merge was kept for the check, and the scratch tree was removed — [rpg-toolkit#2155](https://github.com/KirkDiggler/rpg-toolkit/pull/2155).
