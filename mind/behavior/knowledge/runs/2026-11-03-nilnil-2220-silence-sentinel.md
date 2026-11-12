# Run — 2026-11-03 — nilnil #2220 — behavior silence sentinel

processed: 2026-11-12T165555Z

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at 8622a295. Ruling on the fix shape: [the issue's plan comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).

## Shipped

- `Pick`'s silence answers a wrapped `ErrSilent` instead of `(nil, nil)`; the sentinel joins the module's error block and `Pick`'s doc names it — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Two named tests pin silence: the per-key shape (a table with answers elsewhere stays silent for an unmatched key) and the empty-table shape; the wrap's key is asserted — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).

## Decided

- The sentinel is `ErrSilent`, not a zero-value output with a `Silent` flag: the #2220 ruling — exactly one of a valid object or an error, and an error may be a signal — closes the third state; a caller reads silence with `errors.Is` and a log names what went unanswered — [#2220 ruling](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).
- The raise site wraps with the key (`pick %q: %w`), matching the module's other returns and keeping the sentinel's chain intact; the wrap is pinned by assertion, not just claimed — [table_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/mind/behavior/table_test.go).
- `KeyTime` still answers a hold, not silence: a creature that was given time and did nothing still had its turn; that contract is untouched — [Pick's doc](https://github.com/KirkDiggler/rpg-toolkit/blob/main/mind/behavior/table.go).

## Found

- Only two consumers of `Pick`'s silence exist, both in encounter (`answer.go:195` branches `chosen == nil`; `tabledriver.go:101` maps nil to `ErrBadAnswer`); no in-module caller, no example module, nothing in rpg-api — the adaptation lands in the encounter PR — [the wave table](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).
- The in-module empty-table silence pin already existed (`TestZeroEligibleUnderASocialKeyIsSilence`); it was updated to the sentinel rather than deleted, because it pins a different failure mode than the new per-key test — [table_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/mind/behavior/table_test.go).

## Mistaken

- My first read of the test file said no existing test pinned silence; the suite's red run proved one did. The full gate, not a grep, is the inventory — [table_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/mind/behavior/table_test.go).
- The wave's first per-module scan ran modules in parallel and golangci-lint v2 refused to run, so 11 modules read as clean and the count was first reported as 5 findings in 3 modules; the serial re-run found 11 in 6 — [#2220 comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).