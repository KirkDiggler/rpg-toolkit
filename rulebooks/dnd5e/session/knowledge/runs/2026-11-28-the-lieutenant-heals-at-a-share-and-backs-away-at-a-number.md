# Run — 2026-11-28 — NPC behaviour 4c-2 the Lieutenant heals at a share and backs away at a number

Slice [rpg-project#1250](https://github.com/KirkDiggler/rpg-project/issues/1250) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`eb94e315`](https://github.com/KirkDiggler/rpg-toolkit/commit/eb94e315).

## Shipped

- The pins move to `mind/behavior v0.28.0` and `rulebooks/dnd5e/encounter v0.267.0`; `go mod tidy` moved no other go.mod line — [ede9bff2](https://github.com/KirkDiggler/rpg-toolkit/commit/ede9bff2).
- The site's first row reads `at_most: 30%`; its table gains the `away: enemy` row at 5 and two `not` rows — [f4571aec](https://github.com/KirkDiggler/rpg-toolkit/commit/f4571aec).
- Four new tests in `hit_points_npc_test.go`: backs away at 5, fights at 6, heals then fights at 5 with the use left, and the read-back — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).

## Decided

- In the back-away test the fighter DECLINES the opportunity attack that opens, because the issue asks only that he ends farther from her and an attack would add a die the test does not need; the slice's issue says he has no safe way to slip away — [rpg-project#1250](https://github.com/KirkDiggler/rpg-project/issues/1250).
- Distance is the hex distance between the two members' cells read off the run (`X` is q, `Y` is r), because the record's `moved` beats name steps, not distances — [rpg-project#1250](https://github.com/KirkDiggler/rpg-project/issues/1250).
- The header's only edit to an existing test file is the comment that quoted `at_most: 30`; no assertion was edited — [rpg-project#1250](https://github.com/KirkDiggler/rpg-project/issues/1250).

## Found

- F4, at the new pins before the site moved: exactly `TestAnUnhurtLieutenantDoesNotUseSecondWind` (line 81, "at full hit points the row is not true": an `activated` beat by him) and `TestALieutenantHurtToTheShareUsesSecondWindOnHisNextTurn/one_more_hit_point_and_he_does_not` (line 115, same beat at 9) fail; nothing else — [comment](https://github.com/KirkDiggler/rpg-project/issues/1250#issuecomment-7229568805).
- F5: with the first row alone at `30%`, the whole suite passes, today's tests unedited — [comment](https://github.com/KirkDiggler/rpg-project/issues/1250#issuecomment-7229585252).
- Read-back row 1: "if Lieutenant (lieutenant)'s hit points are at most 30% of its maximum, Lieutenant (lieutenant) activates “second wind”, when it has that ability and can pay for it." — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- Read-back row 2: "if Lieutenant (lieutenant)'s hit points are at most 5, Lieutenant (lieutenant) moves away from the enemy." — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- Read-back row 3: "if NOT (Lieutenant (lieutenant)'s hit points are at most 5) and a walk this turn puts an enemy in Lieutenant (lieutenant)'s reach, Lieutenant (lieutenant) walks toward the enemy." — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- Read-back row 4: "if NOT (Lieutenant (lieutenant)'s hit points are at most 5) and an enemy is in Lieutenant (lieutenant)'s reach, Lieutenant (lieutenant) attacks the enemy." — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- The turn he backs away in, in order: `turn-ended answered window_opened`, then her answer, then `moved` six times, `turn-ended tick`; no `struck` or `missed`. The window is her opportunity attack; he ends 7 cells from her, from 1 — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- Before her answer the turn is paused at `window_opened`, and he has not moved yet; a test that does not answer her window sees no `moved` beat — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- Test 4 (5 hit points, use left, in her reach): beats `activated activation-result struck`; the heal rolls 10 plus 1 and takes him from 5 to 16, above 5 whatever it rolls (the lowest, 2, makes 7), so row 2 is out and rows 3 and 4 are the table: he heals once, then swings, and does not move — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- The fight test (6 hit points) and the heal-then-fight test pass against the site as it stood after the first row alone, so they pin that the new rows stay silent; they were not seen failing first — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
- Proof (a), `at_most: 30` again: the unhurt test fails ("at full hit points the row is not true"), so does the share test at 9, and the read-back test — [f4571aec](https://github.com/KirkDiggler/rpg-toolkit/commit/f4571aec).
- Proof (b), `at_most: 5%` on the away row: the back-away test fails at "he steps out of her reach: she is asked whether to take an opportunity attack" (nothing opens), and the read-back test fails on "of its maximum" — [f4571aec](https://github.com/KirkDiggler/rpg-toolkit/commit/f4571aec).
- Proof (c), `not` dropped from the attack row: the back-away test PASSES 20 of 20 runs, because rows are weighed in order and the away row, first, wins at 5; the `not` rows only make the table read as exclusive, they change no behaviour on this site — [f4571aec](https://github.com/KirkDiggler/rpg-toolkit/commit/f4571aec).
- The suite was not run separately at the old pins (the first run is F4's, at the new pins); at the final head `go build`, `go vet`, the whole suite and `golangci-lint` are green — [f4571aec](https://github.com/KirkDiggler/rpg-toolkit/commit/f4571aec).

## Mistaken

- The back-away test first asserted a `moved` beat right after his turn and failed on the paused opportunity window, not on the site; the test now answers her window — [bfafa092](https://github.com/KirkDiggler/rpg-toolkit/commit/bfafa092).
