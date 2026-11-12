# Run — 2026-11-12 — Composable conditions S16

Slice [rpg-project#946](https://github.com/KirkDiggler/rpg-project/issues/946), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Design and checked plan: [toolkit#2399](https://github.com/KirkDiggler/rpg-toolkit/pull/2399).

## Shipped

- Source checkpoint only: inert configured-effect facts validate matching holdings and own copied output data; canonical fixture content is installed but its new runtime loader is not implemented yet — [scope](https://github.com/KirkDiggler/rpg-project/issues/946).

## Decided

- R6 keeps production composition APIs, fixture data and executable external tests inside the root rulebook module; no examples/items adoption is required — [ruling](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6835450799).

## Found

- The configured-fact suite first fails on absent Frame.HeldEffects/types, then passes with structural validation/deep copies; existing root tests, contributions vet and lint pass — [T1 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/data-authored-effects/docs/ideas/data-authored-effects/plan.md#t1--immutable-configured-effect-facts).

## Mistaken

- None observed in this source checkpoint; remaining tasks are not claimed complete — [slice](https://github.com/KirkDiggler/rpg-project/issues/946).
