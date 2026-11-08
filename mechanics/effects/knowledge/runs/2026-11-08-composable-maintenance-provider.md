# Run — 2026-11-08 — Composable conditions S13 neutral provider

Slice [rpg-project#903](https://github.com/KirkDiggler/rpg-project/issues/903), story [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [d4033752](https://github.com/KirkDiggler/rpg-toolkit/commit/d4033752); approved [plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md).

## Shipped

- Source-stage T1 supplies the neutral lifetime value, owned snapshots/plans, strict restore and scoped end acceptance; T2–T4 and release remain pending — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).

## Decided

- Known duplicate loss notices are idempotent only while membership is live; after end acceptance membership is frozen, so the transport suppresses its re-entrant child notices rather than changing an already-issued plan — [approved end contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#shared-lifetime-and-the-coordinator).
- Restore accepts an empty live subset without recreating declared members, but never a missing/non-live snapshot; current-state data is not fresh construction — [fresh-character ruling](https://github.com/KirkDiggler/rpg-toolkit/pull/2323#issuecomment-6726787390).

## Found

- T1 tests first fail to compile on the absent Lifetime API, then pass with implementation; full module race/vet/lint pass, retaining the generic import seam — [T1 proof contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t1--neutral-lifetime-value).
- A mutant accepting repeated end requests makes TestEndIsAuthorizedOrderedAndIdempotent fail on repeated plans and replaced reasons; exact restoration returns it green — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).

## Mistaken

- None observed in T1; missing-API RED and the deliberate mutation are expected checks, not concealed implementation failures — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).
