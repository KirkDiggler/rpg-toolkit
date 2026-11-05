# Run — 2026-11-05 — NPC behavior S1

Slice [rpg-project#866](https://github.com/KirkDiggler/rpg-project/issues/866) of [#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit main [`3182088c`](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21). Work is staged in module-bounded checkpoints; the issue owns completion.

## Shipped

- Mechanical scalar parameter metadata and producer validation, with no sampling, implicit defaults or template engine — [parameters.go](../../parameters.go).

## Decided

- KirkDiggler approved the composer role and mechanically descriptive parameter metadata; no separate quality-profile framework is required up front — [agreement](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6650897115).
- The checkpoint consumes materialized recipes/tables and keeps saved continuation separate from configuration; exact physical patrol rejoining remains a consumer/content policy rather than a universal kernel choice — [checked plan](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6651019347).

## Found

- Adding published `mind/machine/v0.1.0` selects its required core v0.12.1; the existing behavior module race suite passes with that dependency selection, with no replace or go.work — [plan baseline](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6651019347).

## Mistaken

- The first metadata lint pass found two test lines over 120 columns; wrapping those literals changes no behavior and the owning-module gate is rerun — [parameter tests](../../parameters_test.go).
