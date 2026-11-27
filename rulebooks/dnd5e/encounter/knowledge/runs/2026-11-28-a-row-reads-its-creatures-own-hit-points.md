# Run — 2026-11-28 — NPC behaviour 4b a row's when reads its creature's own hit points

Slice [rpg-project#1245](https://github.com/KirkDiggler/rpg-project/issues/1245) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`bf8406a7`](https://github.com/KirkDiggler/rpg-toolkit/commit/bf8406a7).

## Shipped

- A row's `when` takes `hit_points: { at_most: N }`: compiled, saved (`hit_points`, omitted when nil), cloned deep, said by the site check and beside the row on the record — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).
- `SheetFacts.HitPoints` and `MaxHitPoints`, refused when negative or with no maximum; copied to `MonsterView` on both clocks and into `Facts` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).
- `turntest.Host.Hurt` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).

## Decided

- The author writes the percent (operator ruling 2026-11-28) — [ruling](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7228542408).
- A `hit_points` body with no key is refused "and this writes nothing", a non-mapping "and this is not that"; the issue gave the shape, the empty-body wording is the builder's smallest reading of it — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).
- `hit_points` under a machine state's `when` is left to `guardOf`'s existing refusal, which does not name it (states.go is outside the file list) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1245#issuecomment-7228672503).

## Found

- F6: at the pin alone the suite has exactly one failure, `TestTheCastlesContentHashIsPinned`, `89d972b7…af3a7` to `f00dabc8…c099d`; the production change moves nothing further — [comment](https://github.com/KirkDiggler/rpg-project/issues/1245#issuecomment-7228672503).
- F7: no test pins the two lengthened sentences or a `when`'s saved JSON; `table_internal_test.go:132` passes unedited — [comment](https://github.com/KirkDiggler/rpg-project/issues/1245#issuecomment-7228672503).
- F8: 54 test constructions of `SheetFacts{`, none edited — [comment](https://github.com/KirkDiggler/rpg-project/issues/1245#issuecomment-7228672503).
- `WhenSpec` is used by `WhenListSpec`, `WhenSpec.Not`, `oneWhenOf` and a machine transition's `When` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1245#issuecomment-7228672503).
- Sentences: "if Goblin (gate-patrol)'s hit points are at most 30% of Goblin (gate-patrol)'s maximum" (long), "If hit points at most 30% → holds where it stands." (short), "hit points at most 30%" and "not hit points at most 30%" (record) — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).
- The site-check row keeps its own `table` `row-of` connection only; the condition adds none — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).
- Proofs: (a) whenOf not setting HitPoints fails the compile tests, the picture test and the site-check test; (b) factsFromView not copying fails the 2-of-9 turn case and the world 4-of-9 case; (c) no deep copy fails the clone test; (d) no omitempty fails the WhenData round trip; (e) no validateWhenWords case fails every table-building test with "is not a deed this build knows" — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).

## Mistaken

- The first read-back test matched "of its maximum", but a named creature reads "of <name>'s maximum"; caught by the first run — [slice](https://github.com/KirkDiggler/rpg-project/issues/1245).
