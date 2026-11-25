# Run — 2026-11-23 — a wake says whom it is about S-session known on sight on the filter

Slice [rpg-project#1128](https://github.com/KirkDiggler/rpg-project/issues/1128) of story [rpg-project#1123](https://github.com/KirkDiggler/rpg-project/issues/1123). Base: `rpg-toolkit` `main` at [`436196ea`](https://github.com/KirkDiggler/rpg-toolkit/commit/436196ea0c2d0271c6262148a267235f8999e053).

One line per entry. Every line ends with a link.

## Shipped

- The session requires `rulebooks/dnd5e/encounter` `v0.252.0` (was `v0.251.0`); `go.mod` moves that one line, `go.sum` its two hashes — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).
- `TestKnownOnSight` counts the captain's `behavior_decision` entries on `seen` in the saved log: alice's return adds one, bob's return adds none — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).

## Decided

- none beyond what the issue gave — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).

## Found

- F1 `go.mod` pinned encounter `v0.251.0`; F2 tag `v0.252.0` holds `couldBeWokenBySeeing` (count 1); F3 `TestKnownOnSightWalkSuite` exists at line 42 — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1128#issuecomment-7094845850).
- Before the pin, at `origin/main`, `go test -race -run 'KnownOnSightWalk|TwoStrikesWalk' -count=1 .` passes — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).
- After the pin, with no edit to the site or the scene, the same command passes (TestKnownOnSightWalkSuite 2.00s, TestTwoStrikesWalkSuite 1.86s): the filter does not change the scene — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).
- Proof the assertion can fail: with `go.mod` back at `v0.251.0` the bob assertion fails (expected 1, actual 2: without the filter he is asked and declines); restored to `v0.252.0` it passes — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).
- The log is read from a session test with no non-test change: `fakeEncounters.GetEncounter` returns the saved `Log.Entries`, whose payloads carry `beat`, `member` and `on` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).
- Whole module `go test ./... -count=1 -timeout 20m` passes (59.8s); `go build ./...` and `golangci-lint run ./...` print 0 issues — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).

## Mistaken

- none observed — [slice](https://github.com/KirkDiggler/rpg-project/issues/1128).

— cross-team agent, on behalf of KirkDiggler
