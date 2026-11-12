# Run — 2026-11-03 — nilnil #2220 — world no-goals signal

processed: 2026-11-12T165555Z

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at 8622a295. Ruling on the fix shape: [the issue's plan comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).

## Shipped

- `newTracker` answers `errNoGoals` for a world with no goals, instead of `(nil, nil)`; `New` consumes the signal with `errors.Is` and runs the world without a tracker — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Named test `TestNoGoalsIsASignalNotAFailure` in the package's first internal test file, because the signal never crosses the API and an external test cannot see it — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).

## Decided

- The sentinel is unexported: the news never leaves the package — `New` is its only reader — and exported sentinels in this package are API errors, which this is not — [world.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/world/world.go).
- `New` consumes the signal inline (`err != nil && !errors.Is(err, errNoGoals)`), leaving the tracker nil, rather than guarding with `len(cfg.Goals)` at the call site: the knowledge of what "no goals" means stays with the builder, where its doc already kept it — [#2220 ruling](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).
- The with-goals path is untouched: goals without a clock are refused by `goal.NewTracker`, which is where that message belongs — [newTracker's doc](https://github.com/KirkDiggler/rpg-toolkit/blob/main/world/world.go).

## Found

- Every existing world test constructs a no-goals world through the `build` helper, so "a world with no goals runs" was already pinned — but that pin passes under `(nil, nil)` too, which is why the sentinel needed its own named test — [world_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/world/world_test.go).

## Mistaken

- The new test file was written without a final newline; gofmt caught it at the gate before the commit — [newtracker_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/world/newtracker_test.go).