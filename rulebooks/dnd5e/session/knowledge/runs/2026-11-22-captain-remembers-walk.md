# Run — 2026-11-22 — Captain remembers who lied: a walk site

Slice [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`b9eba855`](https://github.com/KirkDiggler/rpg-toolkit/commit/b9eba855).

## Shipped

- `testdata/captain-remembers-walk.yaml`: a hall and a yard joined by one open door, a Guard Captain with the castle's cut machine, posts and three lines — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- `captain_remembers_walk_test.go`: `CaptainRemembersWalkSuite` with `TestTheCaptainRemembersWhoLiedToHim` and `TestTheOtherCharactersLieIsCaughtToo`, played through `session.Manager` — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).

## Decided

- The d20 script is sixty-four 20s, so any die a wait or a disadvantage roll takes is a 20 and the lie still totals under 30 — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The members' Charisma is 16 (as the castle's tests set it) so the best roll is a real number, and the test asserts the total is at least 20 and the DC is 30 — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The liar stands on the cell beside the door crossing (-3,7) and the friend on (-4,7), so the liar's bar is read at the door's first step; from two cells back she takes the free step first — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).

## Found

- Rests on: F1 holds (four Manager verbs), F2 holds (castle_gate_hold_test.go lines 160-207), F3 holds (three files), F4 holds (castle_free_roam_test.go:61), F5 holds (encounter v0.243.0), F6 holds (social.go:410-427) — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1109#issuecomment-7075370909).
- `validate_only` on the local server refused the first draft twice: a door must be declared under `doorBindings` (`gate-wall-door: {}` starts it open), and a post's door must name a door in the site — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The cell for a hex is x = sqrt3 * (q + r/2), z = 1.5 * r; the hall's cells stay inside the gate wall's span so nobody walks round it — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The captain reaches his gate spot from (-1,4) in one `Wait` — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- Proof 1, remove `not: { noted: caught-lying }` from the lie's `requires` (both lines): step 5 and the reload step fail, "the liar is not offered He burned Ashford again" — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- Proof 2, remove it from the post's `pass`: step 6 fails, the liar is not stopped, takes a step and moves — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- Proof 3, post `pass` is `[ { noted: caught-lying } ]`: step 7 fails (the friend's walk errors); step 6 fails too, because the liar now passes, and she stands in the doorway the friend needs — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The suite takes 1.02 s with `-race` (2.07 s including compile); the whole module `go test ./... -count=1 -timeout 20m` at the final head: both packages ok, 59.6 s — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- `golangci-lint run ./...` prints 0 issues — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).

## Amendment 1 (2026-11-22)

- The `expected` line and the `admitted` note are gone; nothing has to be done first, and `TestAStrangersLieLeavesHimInTheHall` is deleted with them — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The captain reaches his gate after ONE `Wait`, as the operator's steps say — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The second liar is NOT caught: the captain is already in `back-to-gate` and no transition enters it again, so bob's failed lie writes no second note; bob is still offered the lie and the question, only alice lost them. `TestTheOtherCharactersLieIsCaughtToo` asserts this — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- Proof 1, remove the `requires` from `lie`: the liar is still offered the lie, in both tests — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- Proof 2, remove the `requires` from `the-way-in`: the liar is still offered the question — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- Proof 3, delete the captain's `posts:` block: the liar is not stopped at the door, takes a step — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
- The suite takes 1.28 s with `-race`; the whole module `go test ./... -count=1 -timeout 20m` at the final head: both packages ok, 59.6 s; `golangci-lint run ./...` 0 issues — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).

## Mistaken

- The first step 6 walked the liar back from two cells out, so she took the free step before the door barred her and "no step taken" failed; the liar now stands beside the door — [rpg-project#1109](https://github.com/KirkDiggler/rpg-project/issues/1109).
