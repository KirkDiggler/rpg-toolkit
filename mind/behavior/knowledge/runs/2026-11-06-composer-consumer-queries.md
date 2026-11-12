# Run — 2026-11-06 — live consumer provider queries and wake admission

processed: 2026-11-12T165555Z

Part of [S1 adoption #866](https://github.com/KirkDiggler/rpg-project/issues/866), story [#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit [`aca78cdf0714ab249ebd223923dfccf77cf2b3f9`](https://github.com/KirkDiggler/rpg-toolkit/commit/aca78cdf0714ab249ebd223923dfccf77cf2b3f9).

## Shipped

- Implemented the first provider checkpoint: detached per-instance Inspect, constant-time StateOf, atomic overflow-safe DelayWakes and timed-input admission respecting each instance's future wake; not merged and not the live consumer — [Task 1 plan](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/plan-consumer.md#task-1-composer-introspection-and-wake-adaptation).

## Decided

- Keep listener scopes and current timed beat names per instance rather than merge them into a lossy actor-level scope; the caller owns scheduling indexes/opportunity selection, because inspection must not become another clock — [consumer contract](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/consumer-design.md).
- Skip only timed delivery before a future wake; non-timed observation remains deliverable, because one awake role must not spend a sleeping role's continuation — [provider task](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/plan-consumer.md#task-1-composer-introspection-and-wake-adaptation).
- R9 removes backwards compatibility and old-format parity from the consumer plan; this provider adds no source adapter, migration code, editor or global scheduler — [operator ruling](https://github.com/KirkDiggler/rpg-project/pull/877#issuecomment-6698028412).

## Found

- The wake regression failed against unchanged production: expected awake role, got sleeping role; the sleeper advanced early and prevented a data-only timed observation from committing. Focused tests now pass — [scope and acceptance](https://github.com/KirkDiggler/rpg-project/issues/866).
- Missing inspection APIs produced an initial compilation failure, distinct from the wake regression; full module race now passes (root 1.222s, sound 1.017s, stage 1.017s; deed no tests), lint reports zero issues — [provider task](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/plan-consumer.md#task-1-composer-introspection-and-wake-adaptation).

## Mistaken

- The older snapshot test selected at round zero after restoring a wake at nine and dereferenced a now-absent pick; its copy/reload assertions remain, with explicit no-selection at eight and resumed work at nine instead of retaining the accidental early-wake behavior — [snapshot and wake contract](https://github.com/KirkDiggler/rpg-project/issues/866).
