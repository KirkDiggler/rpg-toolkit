# Run — 2026-11-27 — an NPC's turn is paid at the one gate S4 the session tells the encounter what is left of a turn

Slice [rpg-project#1209](https://github.com/KirkDiggler/rpg-project/issues/1209) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`928ee118`](https://github.com/KirkDiggler/rpg-toolkit/commit/928ee118).

One line per entry. Every line ends with a link: a PR, an issue comment, a commit, or a design section anchor.

## Shipped

- The three pins: `rulebooks/dnd5e` v0.240.0, `resolution` v0.84.0, `encounter` v0.262.0; `go mod tidy` moved no other line — [5a0bb764](https://github.com/KirkDiggler/rpg-toolkit/commit/5a0bb764).
- `sheetSeam.readFacts` fills `SheetFacts.Turn` from the stored monster sheet and leaves it nil for a character — [fb00a307](https://github.com/KirkDiggler/rpg-toolkit/commit/fb00a307).
- `strikerSeam.Dash` (new `dash.go`), wired as `Actors.Dasher` with a compile-time proof; the "state between two slices" comments in `striker.go` and `disengage.go`, `bonusDisengageOf` and `AGENTS.md` rewritten — [fb00a307](https://github.com/KirkDiggler/rpg-toolkit/commit/fb00a307).
- `npc_turn_test.go` (internal package): Tests 1, 2, 4, 5, 7 and the door-level forms of 3 and 6 — [72d81b79](https://github.com/KirkDiggler/rpg-toolkit/commit/72d81b79).

## Decided

- Test 3 and Test 6 are proved at the door instead of played, because their played fixtures cannot be built; each is a non-blocking gap comment — [Test 3 gap](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201252948), [Test 6 gap](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201253339).
- Test 8 adds no test of its own: the existing paused-then-resumed NPC strike tests are the fixture, and they pass unedited — [F6 on the first comment](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201019412).
- The Dash seam tests use a skeleton where a swing is needed, because the fixture's `strike` helper names the shortsword and a goblin has none — [npc_pays_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/72d81b79/rulebooks/dnd5e/session/npc_pays_test.go#L92-L96).
- The run in Test 4 is a zombie's, because a goblin's shortbow keeps every foe in reach and a table run is never offered toward a foe in reach — [Test 6 gap](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201253339).

## Found

- F5 at the new pins: build and vet pass; the tests on the issue comment fail, all with `action: 1 needed, 0 left` at the door, and two (`TestCastleActsTranscript`, `Example_theGatehouseActs`) panic and abort the package run, so the list was made with `-skip`; all pass unedited after Design 2 and 3 — [F5 list](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201019412).
- The suite before the pins passes (session 70.0 s) and at the final head passes — [F5 list](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201019412).
- A monster's turn start drops its stored account (`monster.onTurnStart` sets `economy = nil`), so a hand-set two actions never reaches its first ask — [Test 3 gap](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201252948).
- After a swing the encounter still asks a driver once more; a second `Attack` is refused by the encounter itself as a `stayed` beat naming `encounter:intent:attack`, and the door is never reached — [npc_turn_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/72d81b79/rulebooks/dnd5e/session/npc_turn_test.go#L170-L186).
- The encounter's backstop "its action is spent" for a run is not reachable through this module: the session's `Move` twin carries no `Dash`, and the table never offers a run on a spent action — [Test 6 gap](https://github.com/KirkDiggler/rpg-project/issues/1209#issuecomment-7201253339).
- Proofs: (a) `Turn` left nil for a monster fails Tests 1, 2, 3 (door form), 4 and 5, Tests 2, 4 and 5 by the verb aborting at the door with `action: 1 needed, 0 left`, Tests 1 and 3 by `Turn` being nil; (b) no `Dasher` fails Test 4 only (`the story shows the run`, zero run beats); (c) a Dash with no cost fails Tests 4, 6 and 7 and the swing-then-run door test with `activation is invalid: ... a driven Dash is only its payment and carries none` — [72d81b79](https://github.com/KirkDiggler/rpg-toolkit/commit/72d81b79).

## Mistaken

- My first Test 2 asserted the driver was asked the same number of times each round, which the encounter's extra ask after a swing broke; the assertion is gone and the test states the swing count and the sheet — [72d81b79](https://github.com/KirkDiggler/rpg-toolkit/commit/72d81b79).
