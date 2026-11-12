# Run — 2026-11-12 — Composable conditions S16

Slice [rpg-project#946](https://github.com/KirkDiggler/rpg-project/issues/946), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Design and checked plan: [toolkit#2399](https://github.com/KirkDiggler/rpg-toolkit/pull/2399).

## Shipped

- Source implementation, not a release: reusable parts, independent maintenance, immutable definition compilation/binding, configured target facts and the rulebook-local cookbook execute through public APIs; T1–T8 are implemented, operator view and independent review remain pending — [PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2401).

## Decided

- R6 keeps production composition APIs, fixture data and executable external tests inside the root rulebook module; no examples/items adoption is required — [ruling](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6835450799).

- Complete output observation belongs in mechanics/effects: the root consumes the real pushed provider and removes its local clone duplication rather than maintaining a second definition authority — [provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2402).

## Found

- The configured-fact suite first fails on absent Frame.HeldEffects/types, then passes with structural validation/deep copies; existing root tests, contributions vet and lint pass — [T1 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/data-authored-effects/docs/ideas/data-authored-effects/plan.md#t1--immutable-configured-effect-facts).

- The five executed examples prove 12 damage/4 physical damage for both maintained Rage variants, resistance-only and damage-only separation, and the sausage's two attack bearings; root race/vet/lint pass. The workspace pre-commit target still fails in unchanged Core coverage extraction/reporting (bc parse errors, reported 78.0%), not in the owning root gate — [PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2401).

## Mistaken

- A self-attack initially reported the target-side drawback in both actor and target information. A failing assertion now pins one row per bearing and execution agrees with both — [source proof](https://github.com/KirkDiggler/rpg-toolkit/pull/2401).
- A pre-existing test treated every linked-effects ref as concentration without configuration. It now supplies explicit known-empty detail while preserving all its original no-bearing assertions; unknown configured detail is not silently none — [source proof](https://github.com/KirkDiggler/rpg-toolkit/pull/2401).
