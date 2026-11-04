# Run — 2026-11-03 — nilnil #2220 — the session drain

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at 4864537f. Ruling on the fix shape: [the issue's plan comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).

## Shipped

- Session's whole drain: 10 product sites converted, and the `^session/` exclusion deleted from both configs — the module lints clean under its own live config — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Five new sentinels at their seams: `errNoPreview` (previewCastAim, 3 raise sites, consumed at Afford), `errNoSave` (castSave), `errNoTurn` (equipTurn), `errNoUnit` (recordAttack's nothing-landed and not-lone sites, consumed inside attackLanding's Record closure), `errNoRetaliation` (retaliationUnit, 6 caller guards), `errNoWindow` (openWindow, consumed inside both Window closures) — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The workbench fake's `DiscoverySharing` answers `ErrNotFound` — the interface's already-spelled "not chosen" sentinel; the fake was mis-spelled, not the contract — [cmd/session-workbench/main.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/cmd/session-workbench/main.go).
- Internal pins for the three seams a bare call reaches; `errNoTurn` and `errNoWindow` reach their raise sites only through a live write scope, so their consumption is pinned by the world-clock equip tests and the cast resume's posed path — [drains_internal_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/drains_internal_test.go).

## Decided

- The closures consume inside themselves: attackLanding's `Record` and cast's `Window` fall through on the sentinel rather than propagating it into the resume machinery — the sentinel reaching the verb runner would fail verbs that today succeed — [landings.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/landings.go).
- `recordAttack`'s not-refused half respelled in its own test: `ErrorIs(errNoUnit)` replaces `NoError` — the substance ("nothing is recorded, and it is not a failure") is carried by the news itself — [record_train_internal_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/record_train_internal_test.go).
- rpg-api's real `DiscoverySharing` implementation still answers `(nil, nil)` and the manager consumes both spellings — the api side adopting `ErrNotFound` is a follow-up for its next pin PR, not this wave's scope — [exploration.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/exploration.go).

## Found

- Session's 22 findings decompose: 10 product, 12 test-file — the permanent test-fixture exclusion covers all 12 — [#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The interface's spelled contract already carried a not-chosen sentinel (`ErrNotFound`) with the consuming branch in `configuredSharing` — the (nil, nil) was a fake's drift from its own contract — [exploration.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/exploration.go).

## Mistaken

- A scripted edit dropped a `)` from openWindow's default case — caught by `go build` before any test ran — [landings.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/landings.go).
- Two direct pins written for seams that nil-panic before their raise sites (equipTurn's `scope.enc`, openWindow's `poseWindow`) — replaced by the note that names where their consumption is pinned, rather than pins that trap — [drains_internal_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/session/drains_internal_test.go).