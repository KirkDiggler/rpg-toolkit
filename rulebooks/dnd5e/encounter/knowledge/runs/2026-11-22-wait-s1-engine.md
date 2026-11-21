# Run — 2026-11-22 — Wait S1 the engine

Slice [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084) of story [rpg-project#1079](https://github.com/KirkDiggler/rpg-project/issues/1079). Base: `rpg-toolkit` `main` at [`d0262574`](https://github.com/KirkDiggler/rpg-toolkit/commit/d0262574).

## Shipped

- `Encounter.Wait` (`wait.go`), its tests, and the `BeatStayed` doc paragraph; tag `rulebooks/dnd5e/encounter/v0.239.0` after merge — [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084).

## Decided

- A wait is told as a `stayed` entry with cause `encounter:intent:wait`, not a new beat, because the entry already means a spent round that moved nobody — [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084).
- A held member and a member in custody are not refused; downed is the session's to refuse — [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084).

## Found

- Rests on F1-F9 all hold at `d0262574`: F1 worldtime.go:262, F2 clocks.go:1984, F3 cannotmove.go:92 and clocks.go:2233, F4 loot.go:166, F5 sneak.go:164, F6 nothing, F7 lines 66/107/264/601 and testsheets_test.go:19, testworld_test.go:89/176, F8 sheets.go:156, F9 cast_test.go:824 — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1084#issuecomment-7066878895).
- New test times under -race (suite 0.37s total): TestAHeldPlayerWaits, TestAWaitInsideAFightIsRefused, TestAWaitIsARoundForItsActor, TestAWaitSaysSo, TestTwoPlayersWaitingTogetherAreOneRound, TestWaitRefusesWhatItCannotPrice all 0.00s; TestAWaitWakesWhoIsDue 0.01s — [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084).
- Slowest castle test under -race is TestAKnightRefusesAndABlowBringsTheFightBack at 2.28s; none near 10s, no budget entry added — [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084).

## Mistaken

- none observed — [rpg-project#1084](https://github.com/KirkDiggler/rpg-project/issues/1084).
