# Run — 2026-11-27 — an NPC's turn is paid at the one gate S-site a run and a retreat

Slice [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`4f2769c7`](https://github.com/KirkDiggler/rpg-toolkit/commit/4f2769c7).

## Shipped

- A walk site, `testdata/a-run-and-a-retreat-walk.yaml`: one open hall, a Runner 11 cells from the party and a goblin one cell from it — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).
- A played test, `a_run_and_a_retreat_test.go`, three tests on one fighter ending her turn for three rounds (claims 1, 2, 4, 5, 6, 7) — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).

## Decided

- The fighter is a level 1 fighter with 60 of 60 hit points, set on the stored sheet before launch, so three rounds of two attackers cannot down her — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).
- The dice are `secondWindDice` with strike off (every die is min(size, 10)), so every blow lands and the claims count beats and read sheets rather than depend on a roll — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).
- Claim 7 uses `readback.Of(site).NotYetSaid` being empty, the check the custom-item read-back tests use — [custom_items_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/4f2769c7/rulebooks/dnd5e/encounter/dungeonspec/readback/custom_items_test.go#L64).
- Claim 5 is shown by both NPCs acting again in round two after their sheets were spent at the end of round one, because the round boundary and both NPC turns run inside one `EndTurn` and the fixture cannot stop between them — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).

## Found

- `dnd5e:monsters:goblin` is a rulebook monster, not a template base: a template on it is refused at launch with "is not a rulebook base: no such content", so the Skirmisher is the bestiary goblin placed by its own ref and named Goblin in the story — [template.go:172](https://github.com/KirkDiggler/rpg-toolkit/blob/4f2769c7/rulebooks/dnd5e/session/template.go#L172).
- `dnd5e:weapons:mace` compiles as a template action, and the `dash`, `toward`, `attack`, `away` rows and `when: { enemy: dash | walk | reach }` are valid as the issue writes them — [spec.go](https://github.com/KirkDiggler/rpg-toolkit/blob/4f2769c7/rulebooks/dnd5e/encounter/dungeonspec/spec.go#L1920).
- Round one: the Skirmisher swings (scimitar), takes Nimble Escape, walks 6 cells away; the Runner runs 10 cells (every step `encounter:table:dash`) to the cell beside her and is told `stayed ... already in reach`, with no swing — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).
- Round two: the Skirmisher shoots with its shortbow from where it stands and walks away again with no `activated` beat (it left no reach, so nothing asked it to disengage); the Runner swings once with the mace — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).
- Round three: the Skirmisher does not strike (no enemy in reach, the `away` row stays: "nowhere farther from (-8, 4) within 6 cells"); the Runner swings once. Claim 3, a strike by the Skirmisher every round, does not hold; it is reported as a gap and has no test here — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).

## Mistaken

- The first draft of the sheet check read the stored sheets after round three, where the Skirmisher had not acted, and failed; the sheets are now read at the end of each round — [rpg-project#1211](https://github.com/KirkDiggler/rpg-project/issues/1211).
