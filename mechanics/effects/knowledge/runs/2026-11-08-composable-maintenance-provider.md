# Run — 2026-11-08 — Composable conditions S13 neutral provider

Slice [rpg-project#903](https://github.com/KirkDiggler/rpg-project/issues/903), story [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [d4033752](https://github.com/KirkDiggler/rpg-toolkit/commit/d4033752); approved [plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md).

## Shipped

- Source-stage T1 supplies the neutral lifetime value, owned snapshots/plans, strict restore and scoped end acceptance; consumer integration, independent wave review and release remain pending — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).

- Source-stage T2 replaces the sole numeric evaluator result with a scalar/dice union, adds explicitly query-only output kinds and namespaces linked output identity; no compatibility evaluator is retained — [T2 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t2--descriptive-amounts-and-query-only-kinds).

- Source-stage T3 adds typed bag comparisons and checked integer deltas; one transition's acts are staged so overflow commits no earlier write or notification — [T3 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t3--neutral-countdown-operations).

- Source-stage T4 composes independent existing runtimes with local/application terminal scopes, strict restoration, teardown custody and output namespaces; the provider's T1–T4 are implemented, not released — [T4 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t4--local-assembly-of-existing-runtimes).

## Decided

- The concentration consumer needs a bound CanReact query: an output that requests a new save must stop while contributions still answer End. It reads existing attachment/state/assembly permission, adding no lifecycle authority or rulebook dependency; missing-API RED then full provider gates pass — [root consumer](https://github.com/KirkDiggler/rpg-project/issues/904).
- Gate the existing machine-vocabulary dispatch entry, not output contribution handlers or an invented interpreter; retain an ephemeral pre-terminal observation cursor while End runs, with Snapshot still reporting actual machine state — [end findings](https://github.com/KirkDiggler/rpg-toolkit/pull/2327#pullrequestreview-5819824571).
- Narrow restore ownership documentation to copied snapshots and borrowed immutable definitions rather than changing the underlying constructor ownership contract during this fix — [ownership finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2327#discussion_r4507948139).
- Assembly construction refuses mixed holders, a part-supplied second namespace, nested lifecycle hooks and already-terminal application maintenance; the neutral assembly, not a silent override, owns those contracts — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).
- Known duplicate loss notices are idempotent only while membership is live; after end acceptance membership is frozen, so the transport suppresses its re-entrant child notices rather than changing an already-issued plan — [approved end contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#shared-lifetime-and-the-coordinator).
- Restore accepts an empty live subset without recreating declared members, but never a missing/non-live snapshot; current-state data is not fresh construction — [fresh-character ruling](https://github.com/KirkDiggler/rpg-toolkit/pull/2323#issuecomment-6726787390).

## Found

- Independent review reproduced query outputs disappearing inside End and nested machine reactions mutating/detaching siblings before reporting finished; permanent multi-part regressions reproduce both failures before the fix — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2327#pullrequestreview-5819824571).
- T1 tests first fail to compile on the absent Lifetime API, then pass with implementation; full module race/vet/lint pass, retaining the generic import seam — [T1 proof contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t1--neutral-lifetime-value).
- A mutant accepting repeated end requests makes TestEndIsAuthorizedOrderedAndIdempotent fail on repeated plans and replaced reasons; exact restoration returns it green — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).

- T2 begins with missing-API RED; changing instance-prefix selection makes equal recipes collide and fails the identity regression; restored code passes the full module suite — [T2 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t2--descriptive-amounts-and-query-only-kinds).

- T3 missing-API RED becomes green; disabling the overflow guard fails both bounds with wrapped values/partial state and an erroneous terminal transition, then exact restoration passes the full module gates — [T3 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t3--neutral-countdown-operations).

- T4 missing-API RED becomes green; disabling local-completion Changed fails the saved-state notification regression, then restoration passes full module race/vet/lint. Additional constructor assertions first fail on conflicting namespace/dead maintenance and then pass with refusals — [T4 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t4--local-assembly-of-existing-runtimes).

## Mistaken

- The first end test folded only subscribed damage, so it missed query availability and nested sibling reactions. A callback-level ending guard ran after machine mutation; the corrected tests observe queries, state, trackers and damage inside End, not only after teardown — [review reproduction](https://github.com/KirkDiggler/rpg-toolkit/pull/2327#discussion_r4507948135).
- Exact-text documentation edits were rejected before a file change; reading the current block and replacing that exact block resolved them. No runtime failure or test skip was concealed — [S13](https://github.com/KirkDiggler/rpg-project/issues/903).
