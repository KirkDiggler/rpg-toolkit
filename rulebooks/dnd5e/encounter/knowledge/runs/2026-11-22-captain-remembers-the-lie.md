# Run — 2026-11-22 — machine story: the captain remembers who lied to him

processed: 2026-11-25T190427Z

Slice [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`6483132d`](https://github.com/KirkDiggler/rpg-toolkit/commit/6483132d).

## Shipped

- The castle's captain notes `caught-lying` about the one he heard when he enters `back-to-gate`, no longer offers that one the feast line, and his `back-to-gate` post bars them; the compiled golden, the content hash pin, the castle test and the read-back say it — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- The twelve test fixtures that built an engine member from a compiled creature now carry the creature's outcomes beside its posts — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072147953).

## Decided

- The castle test reads the post's bar from a third stored moment, `heroAtTheGate`, the hero walked out to the captain, because a viewer is told only of a holder they see — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072198066).
- The hero's step in through the gate is not asserted: `barred` is written for the open gate door and gate-1's post cells, and no new stepping helper was written, as the amendment says — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072198066).
- The `ready` post is unchanged, so `TestDungeonspec_APassStillTakesAFact` now replaces the two differing passes one by one — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).

## Found

- Rests on F1 to F9 held at `6483132d` — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072123660).
- The refusal the first builder quoted came from the fixtures dropping `Outcomes`, not from the engine's validation — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072147953).
- The compiled golden changes in three places only: the feast line's second requirement, the captain's `Outcomes`, and his `back-to-gate` pass — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- Content hash pin: old `2ad3019497e6fdc2f454941e9b4932a9e5e7f2fa6e22feae82e4650743a41d5c`, new `d3df3a4bfdf3b243c83c4e456ab668dee8e4159cad3fea0c9b03023095bc4650` — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- Existing tests changed because they state the captain as he was: `TestDungeonspec_AnOutcomeCompilesOntoItsInteraction`, `TestDungeonspec_APassStillTakesAFact`, `TestDungeonspec_RecognitionIsRefusedAtTheAuthorsPath`, `TestDungeonspec_ALinesGrantIsRefusedAtTheAuthorsPath`, `TestDungeonspec_HoldsCompilesToPosts`, `TestTheCastleTracesAdmitted` — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- Proof 1: with the `back-to-gate` pass back to `noted: admitted`, the hold assertion at `heroAtTheGate` fails (`does not contain "captain"`); restored — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- Proof 2: with the feast line's `not` removed, the offer assertion fails (at `backAtTheGate`, before `heroAtTheGate` is read); restored — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- `-race` times: new test 1.03 s, slowest of the group `TestMadKingTheCaptainGoesIn` 1.54 s; package `-run 'TestMadKingTheCaptain|TestMadKingAStrangersLie'` 22.8 s before the amendment's walk, 24.8 s after, so the extra walk adds about 2 s to the build — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).
- `go test ./... -count=1` passes at the final head, every package ok; `golangci-lint run ./...` reports 0 issues — [rpg-project#1105](https://github.com/KirkDiggler/rpg-project/issues/1105).

## Mistaken

- The issue did not know the fixtures copy a creature field by field, so the first builder stopped on a refusal the engine did not make; the amendment names the twelve sites and the checklist grep — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072147953).
- The issue asked for a hold read from stored moments that cannot see the captain; the second builder stopped, and the amendment added the walk — [Amendment 2](https://github.com/KirkDiggler/rpg-project/issues/1105#issuecomment-7072198066).
