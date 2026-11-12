# Run — 2026-11-04 — executable guard machine probe

processed: 2026-11-12T165555Z

Story [rpg-project#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: exploration head [`03b1f543`](https://github.com/KirkDiggler/rpg-toolkit/commit/03b1f5430599e2d910dc9e1784f1676eaa93dc03), with production machine code unchanged from main `532f4621`.

## Shipped

- `TestGuardInvestigationSuite`: patrol/clang/search/report/challenge fixture, wrong and unknown inputs, independent instances, and blob reload; five methods with seven table-driven subcases, no production or dependency change — [probe](../../mind/machine/guard_journey_test.go).

## Decided

- KirkDiggler authorized a unit-test exploration, with controlled rolls where randomness is involved; this kernel takes no die, so its first probe controls facts/events/time and leaves exact-roll checks at the behavior-selection consumer — [scope](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6644100984).
- Treat information-only transitions and timed table picks as authored fixture policy, not a ruling for encounter delivery or action costs; do not implement unanswered role/memory/recovery contracts in helpers — [scope](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6644100984).

## Found

- The single-path fixture passes against the existing machine; a new core API is not necessary to express this trace with a supplied vocabulary, but live delivery and concurrent roles remain unproven — [receipt](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6644178404).
- The probe rejects both a missing state write and known-false guards treated as applicable; mutations restored, full module race suite passes, lint reports zero issues, and the uncached scoped hook passes without dependency drift — [receipt](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6644178404).

## Mistaken

- None observed; the initial run was green, so this is recorded as a test-only design probe with negative controls, not as an unimplemented feature made green — [receipt](https://github.com/KirkDiggler/rpg-project/issues/859#issuecomment-6644178404).
