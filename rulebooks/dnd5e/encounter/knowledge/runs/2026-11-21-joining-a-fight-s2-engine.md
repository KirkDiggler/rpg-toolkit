# Run — 2026-11-21 — Joining a fight S2 the engine

Slice [rpg-project#1075](https://github.com/KirkDiggler/rpg-project/issues/1075) of story [rpg-project#1068](https://github.com/KirkDiggler/rpg-project/issues/1068). Base: `rpg-toolkit` `main` at [`8ef15b06`](https://github.com/KirkDiggler/rpg-toolkit/commit/8ef15b06).

## Shipped

- A creature joining a fight under way rolls initiative and takes its place by the rule; both doors (the straggler door and the walk door) go through one `joinFight` — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
- The `transferred` entry for a join carries `round`, `first_round` and, when rolled, `rolls` — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
- The fight keeps and saves each member's initiative total (`EncounterData.Initiative`), and the replay host answers a join from its entry — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).

## Decided

- The rule is the operator's: a joiner always rolls; in round one it acts that round among those yet to act, later it has no turn in the round it joins — [ruling](https://github.com/KirkDiggler/rpg-project/issues/1068#issuecomment-7057992258).
- The totals live with the fight in the encounter and never in the clock, because a total is a rule's number and `play/clock` stays rulebook-agnostic — [rpg-project#1075](https://github.com/KirkDiggler/rpg-project/issues/1075).
- Both doors into a running fight go through one `joinFight` — [rpg-project#1075](https://github.com/KirkDiggler/rpg-project/issues/1075).
- A fight saved without its totals is refused plainly with `ErrSavedByOlderVersion`, never loaded as if nobody rolled — [rpg-project#1075](https://github.com/KirkDiggler/rpg-project/issues/1075).
- `joinFight` validates every roll of the answer before it moves anybody, so a bad second roll never leaves the first joiner in the fight (a private helper choice, `validateRecordedD20` is the same check `transfer` runs) — [rpg-project#1075](https://github.com/KirkDiggler/rpg-project/issues/1075).

## Found

- `go test ./... -count=1` in the module was green before any change (no failures to record) — [rpg-project#1075](https://github.com/KirkDiggler/rpg-project/issues/1075).
- Existing tests changed, fixture only: `clocks_test.go` (eight hand-built `EncounterData` with a bubble) and `defeat_test.go` (one) now also set `Initiative`, one empty entry per bubble — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
- Expected value changed because of the rule: the golden save `data_test.go:186` gains `"initiative":[{}]` after `bubbles` — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
- No existing transcript, scripted die sequence or stand-in roller needed changing in this module (`goblinFirst` and the scripted-dice session tests live in other places or did not break) — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
- A fight of players alone dissolves at the first turn, so the new tests stand a goblin last in the order; its turns are driven on the way past — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
- Castle tests under `-race`: `TestAllyWalkTheVeteranJoinsFollowsAndFights` 1.68 s, `TestAllyWalkNotJoinedTheVeteranHolds` 0.58 s, `TestMadKingAStragglerIsVouchedFor` 1.01 s, `TestMadKingAStragglerMakesTheirOwnCase` 1.02 s, `TestMadKingAStragglerHasOneTry` 1.49 s, `TestMadKingAStragglerWhoWalksAwayKeepsTheRoll` 1.82 s — none crossed 10 s — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).

## Mistaken

- My first tests formed a fight of three players only, which dissolved on the first `EndTurn`; caught by the new tests failing, fixed by standing a goblin in the fight — [https://github.com/KirkDiggler/rpg-toolkit/pull/2509](https://github.com/KirkDiggler/rpg-toolkit/pull/2509).
