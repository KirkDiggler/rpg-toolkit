# Run — 2026-11-22 — Status rounds lifetime S3 resolution

processed: 2026-11-25T190427Z

Slice [rpg-project#1086](https://github.com/KirkDiggler/rpg-project/issues/1086) of story [rpg-project#1080](https://github.com/KirkDiggler/rpg-project/issues/1080). Base: `rpg-toolkit` `main` at [`9df14702`](https://github.com/KirkDiggler/rpg-toolkit/commit/9df14702).

One line per entry. Every line ends with a link.

## Shipped

- `NewBoundary` takes `Lapses` beside crossings and publishes one `StatusLapsedEvent` per lapse after the crossings; `rulebooks/dnd5e` pin v0.223.0 to v0.225.0 — [https://github.com/KirkDiggler/rpg-toolkit/pull/2514](https://github.com/KirkDiggler/rpg-toolkit/pull/2514).
- Four tests in `lapse_status_test.go`, each under 0.02 s, no existing test or fixture edited — [https://github.com/KirkDiggler/rpg-toolkit/pull/2514](https://github.com/KirkDiggler/rpg-toolkit/pull/2514).

## Decided

- The ending is written on the giving, `until: { rounds: 3 }`, as the operator ruled 2026-11-22 — [rpg-project#989 comment](https://github.com/KirkDiggler/rpg-project/issues/989#issuecomment-7066217434).
- Resolution owns its own `Lapse` type and does not read the encounter's new boundary field, because it pins an encounter fifty versions old — [rpg-project#1086](https://github.com/KirkDiggler/rpg-project/issues/1086).
- Lapses are published after crossings, so a status hears the boundaries that came with its lapse first — [rpg-project#1086](https://github.com/KirkDiggler/rpg-project/issues/1086).

## Found

- Rests on: F1 tag v0.225.0 exists, F2 go.mod printed dnd5e v0.223.0 and encounter v0.188.0, F3 and F4 matched the quoted code — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1086#issuecomment-7066933509).
- `status.Compile` refuses a status with no maintenance ("the status has no maintenance"), so a timed test status needs an `ends_on` too — [lapse_status_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/feat/status-rounds-resolution/rulebooks/dnd5e/resolution/lapse_status_test.go).

## Mistaken

- The first test status had no maintenance and failed to compile; fixed by adding `ends_on: run` before the first green run — [https://github.com/KirkDiggler/rpg-toolkit/pull/2514](https://github.com/KirkDiggler/rpg-toolkit/pull/2514).
