# Run — 2026-11-08 — Composable conditions S13 neutral provider

Slice [rpg-project#903](https://github.com/KirkDiggler/rpg-project/issues/903), story [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [d4033752](https://github.com/KirkDiggler/rpg-toolkit/commit/d4033752); approved [plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md).

## Shipped

- Source-stage T1 supplies the neutral lifetime value, owned snapshots/plans, strict restore and scoped end acceptance; T3–T4 and release remain pending — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).

- Source-stage T2 replaces the sole numeric evaluator result with a scalar/dice union, adds explicitly query-only output kinds and namespaces linked output identity; no compatibility evaluator is retained — [T2 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t2--descriptive-amounts-and-query-only-kinds).

## Decided

- Known duplicate loss notices are idempotent only while membership is live; after end acceptance membership is frozen, so the transport suppresses its re-entrant child notices rather than changing an already-issued plan — [approved end contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#shared-lifetime-and-the-coordinator).
- Restore accepts an empty live subset without recreating declared members, but never a missing/non-live snapshot; current-state data is not fresh construction — [fresh-character ruling](https://github.com/KirkDiggler/rpg-toolkit/pull/2323#issuecomment-6726787390).

## Found

- T1 tests first fail to compile on the absent Lifetime API, then pass with implementation; full module race/vet/lint pass, retaining the generic import seam — [T1 proof contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t1--neutral-lifetime-value).
- A mutant accepting repeated end requests makes TestEndIsAuthorizedOrderedAndIdempotent fail on repeated plans and replaced reasons; exact restoration returns it green — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).

- T2 begins with missing-API RED; changing instance-prefix selection makes equal recipes collide and fails the identity regression; restored code passes the full module suite — [T2 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t2--descriptive-amounts-and-query-only-kinds).

## Mistaken

- Exact-text documentation edits were rejected before a file change; reading the current block and replacing that exact block resolved them. No runtime failure or test skip was concealed — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).
