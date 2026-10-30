# Run — 2026-10-30 — Frank's list S5, the not predicate

Slice [rpg-project#719](https://github.com/KirkDiggler/rpg-project/issues/719) of story [rpg-project#714](https://github.com/KirkDiggler/rpg-project/issues/714), under lane [rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722) ruling D3. Base: rpg-toolkit main at [`ed751d26`](https://github.com/KirkDiggler/rpg-toolkit/commit/ed751d26).

## Shipped

- `{ not: <one predicate> }` is a form of the one predicate grammar. It is decoded by hand like the others, refused when it says nothing or two things, and quoted back as the file spells it — [`predicate.go`](../../encounter/dungeonspec/predicate.go).
- The engine carries it as `TriggerNot`, judged where and when its inner predicate is judged, with no third value — [`not.go`](../../encounter/not.go).
- An option's `requires` reads the speaker's knowledge or pockets and negates the answer. A caught character loses the line, and a party member who did not see the theft keeps it (D3) — [`option.go`](../../encounter/option.go), [`not_test.go`](../../encounter/not_test.go).
- A leave ending's `if` reads the world when the last one is out and negates the answer — [`leave.go`](../../encounter/leave.go).
- An until, an arrival or an ending waits on a `not` of a stance. It fires in the fold where the pair turns away from that stance — [`reserve.go`](../../encounter/reserve.go) `onStance`, [`flip.go`](../../encounter/flip.go).
- An ending's `when: { not: { left: gate } }` is the last one out by any other door, judged at the same departure as a bare left — [`holdings.go`](../../encounter/holdings.go) `departureEnding`.
- The blob carries a `not` trigger as kind `not` with its inner in `not`, on endings, untils, arrivals and requirements — [`data.go`](../../encounter/data.go).
- The castle cook has `pilfer` (Sleight of Hand DC 13, failure says "Thief!" and teaches `caught-in-kitchen`). The cook's `pilfer` and `gossip` require `{ not: { fact: caught-in-kitchen } }`. A castletest proves the caught hero loses both and the pedlar still offers his line — [castle cook](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml), [`castle_king_theft_test.go`](../../encounter/castletest/castle_king_theft_test.go).
- The walk site `theft-walk` is cut from the gate walk's frame. A civil cook stands inside the gatehouse with `pilfer` and `gossip`, and a civil pedlar stands outside. Leaving ends on `clean-getaway` if `{ not: { fact: caught-in-kitchen } }` holds, else on `caught`. It is untracked in the franks-list-s1 api worktree and the castle stack's content, with a copy and one scenario test here — [`theft-walk.yaml`](../../encounter/castletest/testdata/theft-walk.yaml), [`theft_walk_test.go`](../../encounter/castletest/theft_walk_test.go).

## Decided

- A `not` is scoped by what it wraps. A `not` of `holds` is legal only on an option's `requires`, and a `not` of `left` only on an ending's `when`. Each is refused elsewhere in the inner form's own sentence — [`grammar.not`](../../encounter/dungeonspec/factions.go).
- A waited-for `not` of a fact, a round or a fall is refused. Each holds from the first frame and can only stop holding, so nothing can fire it. This is the same liveness law that refuses a stance a pair holds from the start — [`notHoldsFromTheStart`](../../encounter/not.go).
- A waited-for `not` of a stance is refused unless the pair holds that stance from the start and something can turn it away — [`validateNotStance`](../../encounter/not.go), [`grammar.notStance`](../../encounter/dungeonspec/factions.go).
- `when: { not: { left: gate } }` is refused when the gate is the only way out, and it needs an outcome like a bare left — [`notLeftLiveness`](../../encounter/not.go).
- A `not` of a `not` is refused as the predicate itself.
- The theft's skill is `sleight-of-hand`, the rulebook's skill id. The issue's `sleight_of_hand` names no rulebook skill or ability, and `resolution.approachModifier` refuses it at the roll — [`skills.go`](../../skills/skills.go).
- The cook's `pilfer` also carries the `not`. The issue says the theft closes the kitchen's "theft and dialogue chances", so a caught thief gets no second try.

## Found

- A `requires` is one predicate. The cook's `ashford` already requires `feast-tonight` and `bribe` requires `holds: purse`, so neither can also carry the `not`. A caught thief who already knew of the feast can still ask about Ashford, and one holding the purse can still bribe. An `all:` form would close this. It is a grammar widening for the lane, not this slice — [castle cook](../../encounter/dungeonspec/testdata/world-builder-v4-castle.yaml).
- The refusal for an option's wrong `requires` changed from "`round` is neither" to "`round` is none of them", and it now names the `not` form. The studio mirrors the old sentence.
- The `before == S` half of the stance edge survives mutation. A pair that turned away has settled, an arrival is spent, and an ending closes the run, so nothing reaches the fold a second time.
- The walk site loads through `dungeonspec.Load` at this head and through rpg-api's `dungeons.NewFileRegistry` with encounter pinned to `a519bc76`. At the api's current pin, v0.154.0, `NewFileRegistry` refuses it ("`not` is not a key this build reads"). That registry loads the whole directory, so the stack will not boot with the site in its content until the api pins this head.
- The api refuses `dnd5e:monsters:commoner`. The castle's `commoner` is a template of the castle file, so the site declares its own as `yard-commoner`.
- rpg-api's `content/the-mad-king.yaml` has to mirror the cook's `pilfer` and the `not` requires before the castle walk — [rpg-api content](https://github.com/KirkDiggler/rpg-api/blob/dev/content/the-mad-king.yaml).

## Mistaken

- The first cut wrote the issue's `sleight_of_hand` into the castle and the tests. Every test passed, because the engine never reads the skill name, and only the session's roll would have refused it. The rulebook's `skills.All` was read before the commit — [`skills.go`](../../skills/skills.go).
