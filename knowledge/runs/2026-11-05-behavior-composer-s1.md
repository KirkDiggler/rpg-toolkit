# Run — 2026-11-05 — NPC behavior S1

processed: 2026-11-05T201556Z

Slice [rpg-project#866](https://github.com/KirkDiggler/rpg-project/issues/866) of [#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit main [`3182088c`](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21). Work is staged in module-bounded checkpoints; the issue owns completion.

## Shipped

- Mechanical scalar parameter metadata and producer validation, with no sampling, implicit defaults or template engine — [parameters.go](../../mind/behavior/parameters.go).

- Composer observation/ordered selection and strict private-state reload, using the actual machine and picker rather than a test coordinator — [composer.go](../../mind/behavior/composer.go), [composer_data.go](../../mind/behavior/composer_data.go).

- Real patrol/investigation composition fixture: both d200 branches, addressed arrival, retained pending leg, parameter-driven duration, budget holds and JSON reload — [scenario](../../mind/behavior/patrol_composition_test.go).

- Teammate-facing architecture walkthrough: overview, machine/composer/state zoom-ins, real guard trace, ownership and integration limits; no new runtime contract or live-adoption claim — [guide](../../docs/architecture/components/mind.md).

## Decided

- KirkDiggler approved the composer role and mechanically descriptive parameter metadata; no separate quality-profile framework is required up front — [agreement](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6650897115).
- The checkpoint consumes materialized recipes/tables and keeps saved continuation separate from configuration; exact physical patrol rejoining remains a consumer/content policy rather than a universal kernel choice — [checked plan](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6651019347).

- No-candidate holds keep the requesting continuation unchanged; explicitly selected Hold rows can progress. This preserves the distinction between receiving an opportunity and finding eligible work — [composer tests](../../mind/behavior/composer_test.go).

- Completion feedback uses the selected instance ID; an empty Observe address remains a broadcast. This follows private continuation ownership and avoids advancing sibling attachments of one definition — [addressed observation test](../../mind/behavior/composer_test.go).

- The run record lives at the repo root because this PR also changes the root dependency map and repository-level plan; the map now names the behavior-to-machine dependency — [map](../map.md).

## Found

- Adding published `mind/machine/v0.1.0` selects its required core v0.12.1; the existing behavior module race suite passes with that dependency selection, with no replace or go.work — [plan baseline](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6651019347).

- Restored full-module race tests and lint pass; temporary deferred-progress and upper-bound mutations fail the intended assertions and are not committed — [composer tests](../../mind/behavior/composer_test.go), [parameter tests](../../mind/behavior/parameters_test.go).
- Two-role/one-row decision microbenchmark: 13,554 ns/op, 4,003 B/op and 34 allocations on this host, including defensive copies but excluding real RNG, geometry and execution; not a keep performance claim — [benchmark](../../mind/behavior/composer_bench_test.go).

- Independent review at code head `38dae150` found no Critical, Important or Minor findings, ran the focused suites, independently killed/restored the wrong-bag-kind loader mutant and verified head-specific CI — [published review](https://github.com/KirkDiggler/rpg-toolkit/pull/2277#pullrequestreview-5781061951).

- Architecture guide diagrams render successfully as four SVGs in isolated headless system Chrome and were visually checked; the configuration/progress diagram uses a vertical layout to keep labels readable. No browser setup or runtime changes — [guide](../../docs/architecture/components/mind.md).

## Mistaken

- The initial selected-answer copy happened after a caller fact callback could mutate the stored table; a regression observed expected X=4 versus actual X=99. Copying selected rows before the picker and detaching its result closes both sides of that boundary — [alias regression](../../mind/behavior/composer_test.go).
- Completion routing was initially only broadcast; the consumer's returned instance ID needs an addressed observation path so sibling attachments do not advance together. The targeted instance and unknown-address refusal are now exercised — [completion test](../../mind/behavior/composer_test.go).
- Lint caught overlong test/comment lines and an unnecessary alias conversion; formatting and the full owning-module gate were rerun rather than bypassed — [tests](../../mind/behavior/parameters_test.go).
