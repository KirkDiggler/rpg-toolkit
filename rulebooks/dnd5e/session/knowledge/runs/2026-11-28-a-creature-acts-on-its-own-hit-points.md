# Run — 2026-11-28 — NPC behaviour 4c a creature acts on its own hit points

Slice [rpg-project#1246](https://github.com/KirkDiggler/rpg-project/issues/1246) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`f87b7e0d`](https://github.com/KirkDiggler/rpg-toolkit/commit/f87b7e0d).

## Shipped

- `mind/behavior` pinned to v0.27.0 and `encounter` to v0.266.0; `go mod tidy` moved no other line (go.sum only swapped those two modules' hashes) — [eb31b5b2](https://github.com/KirkDiggler/rpg-toolkit/commit/eb31b5b2175f8d650147ab275cf8c5ab30777d7d).
- `readFacts` tells a monster's `HitPoints` and `MaxHitPoints` off its stored sheet; a character tells none — [68e74b1a](https://github.com/KirkDiggler/rpg-toolkit/commit/68e74b1a12ac1346c9833447b327eaff0a73b0b0).
- The two `omitted` entries for `encounter.MonsterView.HitPoints` and `MaxHitPoints` — [5472e86f](https://github.com/KirkDiggler/rpg-toolkit/commit/5472e86f).
- The site `lieutenant-heals-when-hurt-walk` and `hit_points_npc_test.go` — [issue comment](https://github.com/KirkDiggler/rpg-project/issues/1246#issuecomment-7228992610).

## Decided

- The twin omits both fields for Table's reason, as the issue's Design 3 wrote — [Design 3](https://github.com/KirkDiggler/rpg-project/issues/1246).
- Test 3 holds the memo by the within-turn count of one `activated` beat; the condition and the spent use cannot be told apart on this site, so the next-turn assertion proves both together — [issue Test 3](https://github.com/KirkDiggler/rpg-project/issues/1246).

## Found

- F5 after the pins alone: exactly one failure, `TestConvertSuite/TestEveryInnerFieldIsCarriedOrJustified/MonsterView`, naming the two new fields — [comment](https://github.com/KirkDiggler/rpg-project/issues/1246#issuecomment-7228992610).
- F6 (a) printed nothing; (b) one code comment (`conditions_seen.go:103`); (c) nine test files, none names `encounter.SheetFacts` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1246#issuecomment-7228992610).
- His maximum with `6d8` is 27 hit points; the row fires at 8 or fewer (8/27 = 29.6%, 9/27 = 33.3%) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1246#issuecomment-7228992610).
- The turn he heals in, in order: turn-ended, answered, activated, activation-result, answered, moved, moved, answered, struck, heard, answered, stayed, turn-ended, tick — [Test 2](https://github.com/KirkDiggler/rpg-toolkit/blob/feat/a-creature-acts-on-its-own-hit-points/rulebooks/dnd5e/session/hit_points_npc_test.go).
- The read-back: "`time` row 1 of `lieutenant-policy`: if Lieutenant (lieutenant)'s hit points are at most 30% of its maximum, Lieutenant (lieutenant) activates “second wind”, when it has that ability and can pay for it." — [Test 6](https://github.com/KirkDiggler/rpg-toolkit/blob/feat/a-creature-acts-on-its-own-hit-points/rulebooks/dnd5e/session/hit_points_npc_test.go).
- Proof a (tell neither number): tests 2 to 5 fail, "should have 1 item(s), but has 0" for the `activated` beat — [commit 68e74b1a](https://github.com/KirkDiggler/rpg-toolkit/commit/68e74b1a12ac1346c9833447b327eaff0a73b0b0).
- Proof b (tell his maximum as his hit points): the same four fail the same way — [commit 68e74b1a](https://github.com/KirkDiggler/rpg-toolkit/commit/68e74b1a12ac1346c9833447b327eaff0a73b0b0).
- Proof c (hit points always 0): test 1 fails, "at full hit points the row is not true", an `activated` beat by him; test 2's one-more case fails too — [commit 68e74b1a](https://github.com/KirkDiggler/rpg-toolkit/commit/68e74b1a12ac1346c9833447b327eaff0a73b0b0).
- The suite passes at the old pins except the one expected failure, and passes whole at the final head; build, vet and `golangci-lint run ./...` (0 issues) are clean — [comment](https://github.com/KirkDiggler/rpg-project/issues/1246#issuecomment-7228992610).

## Mistaken

- The first draft of test 1 looked for his steps by `actor`; a step names its mover as `member`, so it failed for a reason of its own until the helper read both — [issue](https://github.com/KirkDiggler/rpg-project/issues/1246).
