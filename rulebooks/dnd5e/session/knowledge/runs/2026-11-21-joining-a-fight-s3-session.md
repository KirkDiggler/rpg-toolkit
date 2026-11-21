# Run — 2026-11-21 — Joining a fight S3 the session

Slice [rpg-project#1077](https://github.com/KirkDiggler/rpg-project/issues/1077) of story [rpg-project#1068](https://github.com/KirkDiggler/rpg-project/issues/1068). Base: `rpg-toolkit` `main` at [`f5a0d773`](https://github.com/KirkDiggler/rpg-toolkit/commit/f5a0d773).

## Shipped

- A `transferred` beat to the turn clock folds to a `fight_joined` clock entry (`FightJoined`: member, roll or none, round joined, round of first turn); pinned `encounter v0.238.0` — [https://github.com/KirkDiggler/rpg-toolkit/pull/2510](https://github.com/KirkDiggler/rpg-toolkit/pull/2510).
- Tests for a join with a roll, a placed join, a leave, a roll that does not replay, and one through Manager — [https://github.com/KirkDiggler/rpg-toolkit/pull/2510](https://github.com/KirkDiggler/rpg-toolkit/pull/2510).

## Decided

- A join is its own clock mark in the act stream and folds from the record entry as every clock entry does; the session decides nothing about placement or waiting — [rpg-project#1077](https://github.com/KirkDiggler/rpg-project/issues/1077).
- The transfer is read by its own decode, `joinPieces`, because `to` is a position on `window-opened` in the shared payload struct and a clock name here; a private helper in a named file, accepted by the lead — [rpg-project#1077](https://github.com/KirkDiggler/rpg-project/issues/1077).

## Found

- Fixtures changed per Amendment 1 (empty `Initiative` entry per hand-built bubble, no assertion changed): `authorTurnClock` in `launchscene_test.go` and in `launchscene_internal_test.go`; no other hand-built `Bubbles` needed it — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1077#issuecomment-7059010354).
- Existing expected value changed: `acts_test.go` moves `BeatTransferred` from record-only to `FamilyClock`, a fight join now has an act — [rpg-project#1077](https://github.com/KirkDiggler/rpg-project/issues/1077).
- No die script and no stand-in roller needed editing; `TestMoveSuite/TestAWalkerSpottedByAFightsHostileJoinsBeforeTheNextStep` expected order changes from bob, ogre, alice to bob, alice, ogre, because the scene is a three-way tie at 12 (10 + Dexterity 2 each) and a tie goes to the lower id, so the order shows the tie rule, not an out-roll — [Amendment 3](https://github.com/KirkDiggler/rpg-project/issues/1077#issuecomment-7059209772).
- The encounter v0.238.0 refuses to load a fight saved with no `Initiative`, so every hand-built fight in a test needs the entry; 136 subtests failed on it before the fixture edit — [gap](https://github.com/KirkDiggler/rpg-project/issues/1077#issuecomment-7059000311).
- Castle tests under `-race` on a loaded machine (seconds): InAFight 18.5, FreeRoam 16.6, ActsTranscript 11.0, TheTroupeIsToldNoHold 10.2, TroupeActsTranscript 9.9, HeroInLineWithTheGate 9.8, ThePostHoldsTheGatesOpening 9.7, HeroAtTheEastEnd 9.2, AClickOnTheGuardIsABump 9.2, AWatcherIsSentOnlyWhoAnswered 8.8; every time was about 7 percent up on the list, on a loaded machine, and `TestCastleTheTroupeIsToldNoHold` at 10.2 against 9.7 on [toolkit#2477](https://github.com/KirkDiggler/rpg-toolkit/issues/2477)'s list is the one that crosses 10 s; no time-budget or skip entry was added, and CI's own time check is the answer — [toolkit#2477](https://github.com/KirkDiggler/rpg-toolkit/issues/2477).
- `golangci-lint run ./...` reports 0 issues — [https://github.com/KirkDiggler/rpg-toolkit/pull/2510](https://github.com/KirkDiggler/rpg-toolkit/pull/2510).

## Mistaken

- I started a second full castle run after the first had printed its times, which was wasted work and was stopped — [https://github.com/KirkDiggler/rpg-toolkit/pull/2510](https://github.com/KirkDiggler/rpg-toolkit/pull/2510).
