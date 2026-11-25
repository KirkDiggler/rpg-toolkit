# Run — 2026-11-24 — a creature hails for itself S1 its own machine

processed: 2026-11-25T190427Z

Slice [rpg-project#1140](https://github.com/KirkDiggler/rpg-project/issues/1140) of story [rpg-project#1139](https://github.com/KirkDiggler/rpg-project/issues/1139). Base: `rpg-toolkit` `main` at [`e4c676d1`](https://github.com/KirkDiggler/rpg-toolkit/commit/e4c676d1).

## Shipped

- `ownhail.go`: a second pass at the end of a step, after the conversation's, wakes each creature that hails for itself with `approached` at once; no address, no `addressed` beat, nothing swept — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- `StepOutput.Hailed` names each hailer whose step fired (a pick ran or a state changed), nearest first then by id — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- The `approached` row's Short and Long say the creature's own rule first and the conversation's second — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- Design 4 (refuse an `approached` step with no `within`) is NOT shipped; reported as a non-blocking gap — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1140#issuecomment-7117199735).

## Decided

- A hailer's distance is read from `InstanceStatus.Listens` per attached instance, the larger `within`; any `approached` listen with no scope or no cell `within` makes the creature no hailer (the machine leaves differing scopes unscoped) — [Design 1](https://github.com/KirkDiggler/rpg-project/issues/1140).
- "Fired" is a pick that resolved (`reactionPicked`) or a changed state across every attached instance, read before and after the wake — [Design 2](https://github.com/KirkDiggler/rpg-project/issues/1140).
- The sight refresh after a hailer's answer is done exactly as `hail` does it (only when a pick or a blocking turn happened) — [Design 1](https://github.com/KirkDiggler/rpg-project/issues/1140).
- Existing tests changed: `TestCastleSuite/TestTheHailIsTheEngines` (pins the row's Long) and the `hail-site` goldens `hail-site.folded.txt`, `hail-site.readback.json`, `hail-site.readback.txt` regenerated with `-update` (they print the row's Short and Long; the diff is those lines only). Goldens are fixtures, not tests; I took them to be the same allowed kind and say so — [Allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1140).
- The pass is bounded by the listener index: it returns at once when nobody listens for `approached`, walks `listeners.For(approached)` not every member, and asks `carriesAnApproach` before reading a machine; `TestOwnHailNobodyListeningIsNeverAsked` pins it — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1140#issuecomment-7117220384).

## Found

- Rests on: F1 to F9 hold at `e4c676d1` (F7 prints 6); the machine drops each listen's scope when it lists a member (`relistBehavior` stores `machine.Listen{Beat}` alone), so `listensFor` always answers no scope and the distance must be read from `Inspect` — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1140#issuecomment-7116881400).
- Before the change, with `Hailed` not yet a field, the package did not compile ("out.Hailed undefined"); with only the field added, the failing tests are CrossesIn, MovesOn, AgainAfterLeaving, EachPerson, EachGuardForHimself, TiesGoByID, HisWhenDecides (bob), WritesNoAddress, TheWordsSayBothWays, the dungeonspec refusal tests and the readback words test, each "Not equal" on `Hailed` or the old words — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- The tests of standing near, only whom he sees, down, in a fight, a carrier and no-within pass before the change by nature: they guard what must stay, and the three proofs show the first, second and fourth can fail — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- Proof 1, drop "started beyond": `TestOwnHailNotForStandingNear` fails. Proof 2, drop "sees the mover": `TestOwnHailOnlyWhomHeSees` fails. Proof 3, let a carrier through: `TestOwnHailACarrierKeepsTheConversationsRule` fails; each restored — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- A fight takes the whole roster at setup (alice, the guard and the raider are all in it), so "the hailer is in a fight" and "the mover is in a fight" cannot be told apart by one fixture; the test asserts the pass declines — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- Distances on this hex grid are not the coordinates: odd rows shift one cell, so a placed cell is read back with `GetEntityPosition`; a one-cell step can only cross a distance exactly (end at `within`, start at `within + 1`), so "nearest first" is shown with two different `within`s — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- Whole-module suite on `origin/main` before the change: `ok` for `encounter` in 56.9s, no failure. At the final head: no failure; `go test -race -run 'OwnHail|Address|Hail' ./...` ok, 1m10s wall with compile; `golangci-lint run ./...` 0 issues — [slice](https://github.com/KirkDiggler/rpg-project/issues/1140).
- Design 4 as written refuses the `hail-site` file with its gate's approach removed, which `TestTheSeizureSaysTheHailsDistance` loads — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1140#issuecomment-7117199735).

## Mistaken

- The first greps were run from the main checkout rather than the worktree; the facts were re-run in the worktree before any edit, and the commands now carry the worktree path — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1140#issuecomment-7116881400).
