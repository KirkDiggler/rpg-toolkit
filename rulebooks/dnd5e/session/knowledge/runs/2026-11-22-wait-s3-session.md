# Run — 2026-11-22 — Wait S3 the session

Slice [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088) of story [rpg-project#1079](https://github.com/KirkDiggler/rpg-project/issues/1079). Base: `rpg-toolkit` `main` at [`f4229601`](https://github.com/KirkDiggler/rpg-toolkit/commit/f4229601).

## Shipped

- `Manager.Wait` (`WaitInput`, `WaitOutput`), built as Loot is, and the `encounter v0.239.0` pin; `StayAct` doc says it also carries a wait — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).

## Decided

- The act is `wait`; its body is the existing `StayAct` with cause `encounter:intent:wait`, folded by the existing stayed arm — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- Downed is refused here, where the sheets are; every other refusal is the encounter's — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- The waiter's own waiting roll refuses the wait (Loot's rule via `openForChangeBy`), not every waiting roll (Rest's) — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- The order is roster read (`ClockOf`), downed gate, `Wait`, commit; this replaces the builder's first reorder (encounter first), because that spent a world round on a downed member before discarding it — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1088#issuecomment-7068539102).
- The fight-refusal test reaches a fight by sneaking into the hostile goblin's zone and declaring off, the existing fixture that forms one in `SneakSuite` — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).

## Found

- Rests on: F1 holds (tag present, 1 `Wait`, pin was `v0.238.0`), F2 holds (lines 97, 101, 106, 111, 119), F3 holds (`standing.go:268`, `move.go:292`), F4 holds (`act_family.go:221`, `:78`), F5 holds (`locking_coverage_test.go:28`), F6 holds (`hold_status_test.go:34`, `sneak_test.go:39`, `death_test.go:42`, `attack_test.go:113`), F7 holds (`ErrNotMember`, `ErrClosed`, `ErrInBubble`) — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- Baseline `go test ./...` before any change, after the pin: both packages ok, no failures; after: same, 62 s — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- New test times: `TestAHeldPlayerWaitsAndARoundPasses` 0.02 s, `TestAWaitIsRefusedInAFight` 0.01 s, `TestAWaitKeepsSneakingAndIsToldAsAWait` 0.00 s, `TestWaitRefusesAStranger` 0.00 s, `TestADownedMemberCannotWait` under 0.01 s — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- `go test -race -run TestCastle` shows existing castle tests over 10 s (`TestCastleApproach_InAFight` 18.80 s, `TestCastleApproach_FreeRoam` 17.10 s, `TestCastleActsTranscript` 11.07 s, `TestCastleTheTroupeIsToldNoHold` 10.28 s, `TestCastleTroupeActsTranscript` 10.11 s); none touch Wait; reported as a gap, no budget entry added — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).
- `golangci-lint run ./...` prints 0 issues — [rpg-project#1088](https://github.com/KirkDiggler/rpg-project/issues/1088).

## Mistaken

- The first `Wait` followed the issue's order and the stranger test failed with invalid session data instead of `ErrNoMember`; my fix (encounter first) spent a round before the downed refusal, and amendment 1 replaced it with the roster read first — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1088#issuecomment-7068539102).
