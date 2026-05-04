---
name: dice module
description: Dice rolling infrastructure — Roller interface, dice notation parser, mockable randomness for tests
updated: 2026-05-04
confidence: high — verified by reading dice/*.go and rpg-api's import callsites per audit 049
---

# dice module

**Path:** `dice/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/dice`
**Grade:** B+ (no scorecard yet, baseline grade)

The randomness primitive. Every roll in the toolkit (attack rolls, damage,
saves, ability checks, initiative) goes through this module. Crypto-backed by
default; mockable in tests via the `dice/mock` sub-package.

## What rpg-api consumes

Per audit Section 1, rpg-api imports `dice` from 3 production files plus 3
test files:

| Symbol | Where rpg-api uses it |
|---|---|
| `dice.Roller` | `internal/orchestrators/dice/orchestrator.go` (field type) |
| `dice.NewRoller` | `internal/orchestrators/dice/orchestrator.go` (constructor) |
| `dice/mock.MockRoller`, `dice/mock.NewMockRoller` | encounter and dice tests |

rpg-api consumes the toolkit's dice as a **library** (the `Roller` interface).
The "dice service" inside rpg-api (with `Service`, `RollDiceInput`,
`RollDiceOutput`, `GetRollSession`, etc.) is rpg-api's own service shape — it
wraps the toolkit `Roller` and adds session persistence, but those Service
types are defined in `internal/orchestrators/dice/types.go`, not in this
module.

The boundary: rpg-api owns dice-session persistence (Redis-backed); the
toolkit owns randomness and notation parsing.

## Module surface

Verified by `grep -nE '^func [A-Z]|^type [A-Z]' dice/*.go` (excluding tests):

| File | Exported |
|---|---|
| `roller.go` | `Roller` interface, `CryptoRoller` struct |
| `roller_new.go` | `NewRoller()`, `NewMockableRoller(r Roller)` |
| `pool.go` | `Pool`, `Spec`, `NewPool`, `SimplePool` |
| `notation.go` | `ParseNotation`, `MustParseNotation` |
| `lazy.go` | `Lazy`, `NewLazy`, `NewLazyWithRoller`, `LazyFromNotation` |
| `modifier.go` | `Roll`, `NewRoll`, `NewRollWithRoller`, `D4`, `D6`, `D8`, `D10`, `D12`, `D20`, `D100` |
| `result.go` | `Result` |
| `errors.go` | dice-specific errors |
| `mock/mock_roller.go` | `MockRoller`, `NewMockRoller`, `MockRollerMockRecorder` |

## The Roller interface

```go
type Roller interface {
    // Roll returns a random number from 1 to size (inclusive).
    Roll(ctx context.Context, size int) (int, error)

    // RollN rolls count dice of the given size.
    RollN(ctx context.Context, count, size int) ([]int, error)
}
```

`CryptoRoller` is the production implementation (uses `crypto/rand`).
`NewRoller()` returns a `*CryptoRoller`. `NewMockableRoller(r Roller)` lets
test code inject any `Roller` implementation — typically a `MockRoller` from
the `mock/` sub-package.

The `RollN` contract honors context cancellation (returns
`dice: rolling cancelled` if ctx is done mid-rolls).

## Dice notation

`ParseNotation(notation string)` parses standard dice strings (`"3d6"`,
`"1d20+5"`, `"4d6kh3"` — keep highest 3) into a `*Pool`. The Pool can be
rolled to produce a `Result`.

`MustParseNotation` panics on parse error — for compile-time-known dice
strings only.

## Lazy rolls

`Lazy` defers the actual roll until `Result()` is called. Useful when a
modifier ("Bless adds 1d4") needs a fresh roll *each time* it's applied —
constructing a `Lazy` once and re-rolling it on each use is the toolkit's
standard pattern for "fresh dice per use" bonuses.

`LazyFromNotation(notation)` is the convenience constructor.

## Mock package

`dice/mock/mock_roller.go` is gomock-generated. Tests construct `MockRoller`
via `NewMockRoller(ctrl)` and use the `*MockRollerMockRecorder` to assert
expectations:

```go
mockRoller := mock_dice.NewMockRoller(ctrl)
mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(15, nil)
```

rpg-api uses this in encounter and dice-orchestrator tests to make attack
rolls deterministic.

## go.mod status

Clean. No replace directives.

## Known gaps

- The lazy/pool/notation surfaces have unit tests, but their interaction with `Roller` injection is not exhaustively tested across all combinators (e.g., `Lazy` + `MockableRoller` + parser-edge-case strings).
- No documented behavior contract for what happens when `RollN` receives `count: 0` — current implementation returns an empty slice, which is reasonable but unstated.

## Verification

```sh
# Module surface (production files)
grep -nE "^func [A-Z]|^type [A-Z]" /home/kirk/personal/rpg-toolkit/dice/*.go | grep -v _test

# rpg-api's dice imports
grep -rn '"github.com/KirkDiggler/rpg-toolkit/dice' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go"

# Production file count vs mock file count
grep -rln '"github.com/KirkDiggler/rpg-toolkit/dice"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | grep -v _test | wc -l    # expect 3 (or close)
grep -rln '"github.com/KirkDiggler/rpg-toolkit/dice/mock"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l               # expect 3
```
