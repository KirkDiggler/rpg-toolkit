# Run — 2026-11-23 — a wake says whom it is about S3 known on sight walk site

Slice [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127) of story [rpg-project#1123](https://github.com/KirkDiggler/rpg-project/issues/1123). Base: `rpg-toolkit` `main` at [`6b302849`](https://github.com/KirkDiggler/rpg-toolkit/commit/6b302849).

## Shipped

- `testdata/known-on-sight-walk.yaml`, cut from `two-strikes-walk.yaml`, key and room and scene name `Known on Sight`, no post and no outcomes — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- `known_on_sight_walk_test.go`: `TestKnownOnSight` (the six steps with a reload) and `TestTheOrderDoesNotMatter` — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- `go.mod` and `go.sum`: `rulebooks/dnd5e/encounter` v0.248.0 to v0.251.0, only that requirement line and its two sum lines moved — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).

## Decided

- His door spot is (-4,6), inside the hall beside the doorway (-3,6); alice's seen cell is (-3,7) and her unseen cell is (-4,7) — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Bob comes back to the doorway (-3,6), which the engine also says he sees, because alice stands on (-3,7) and two members cannot share a cell — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).

## Found

- F1 to F7 all hold at `6b302849` — [Rests on comment](https://github.com/KirkDiggler/rpg-project/issues/1127#issuecomment-7094443347).
- Cells, each asked of the engine with `Encounter.SeesWithin("captain", "alice", 1000000)` through `LoadWorldForTest` with him on (-4,6): doorway (-3,6) sees, outside (-3,7) sees, (-4,7) does not, (-5,7) does not, (-6,7) does not, (-7,7) does not — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Check (a) holds: he sees through the doorway and the door cells, and not through the gate wall to the yard cell beside it — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Check (b) holds: the `say` row picked on `time` reaches the session as a speech act of his with the words "I said out." — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Check (c) holds: `seen` reaches him each time alice goes out of and back into his sight (twice), and bob's returns wake nothing because the `when` reads the note — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Proof 1, the `on: topic` step turned into a `heard` step: "he walked to his door spot within ten rounds" fails — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Proof 2, the `on: seen` step removed: "the captain says 'I said out.'" fails at all three return steps — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Proof 3, the `when` removed from the `on: seen` step: "bob steps back into sight: the captain says nothing" fails (he said "I said out." three times) — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Suite time with `-race`: 2.02 s (target under 5 s) — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
- Whole module `go test ./... -count=1 -timeout 20m` passes (session 60.5 s); `golangci-lint run ./...` reports 0 issues — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).

## Mistaken

- The first draft walked bob from alice's start cell and sent both to the same seen cell; the run showed bob starts on (-3,3) and a member cannot step onto an occupied cell, so the test now walks from where each stands and bob returns to the doorway — [rpg-project#1127](https://github.com/KirkDiggler/rpg-project/issues/1127).
