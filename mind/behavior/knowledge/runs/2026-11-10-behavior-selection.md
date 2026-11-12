# Run — 2026-11-10 — Behavior selection S1

processed: 2026-11-12T165555Z

Slice [rpg-project#918](https://github.com/KirkDiggler/rpg-project/issues/918) of [story#917](https://github.com/KirkDiggler/rpg-project/issues/917). Base mainba6d40f2.

## Shipped

- Implemented in a provider PR, not yet merged: captured evaluation, weighted/explicit choices, real Pick and Deal adoption, guard/dialogue examples; no new module, dependency, persistence or API/web contract — [PR#2351](https://github.com/KirkDiggler/rpg-toolkit/pull/2351).

## Decided

- Kirk approved forming `mind/behavior/selection` inside the behavior module, one way forward rather than a new alternatives round; original selectables remains untouched — [direction](https://github.com/KirkDiggler/rpg-project/issues/917).
- The delegated numeric choice is nonnegative integer shares with ordered integer multipliers, not several modifier modes; behavior passes its raw temperament percentage as the factor so existing die sizes remain exact — [R3](../../../../docs/ideas/behavior-selection/design.md#law).

- Adopt the existing sorted temperament Deal as well as Pick because both contained weighted landing loops; valid sorted-word/profile/die outputs remain unchanged, and empty/overflowing/bad-face input refuses explicitly — [Task3 seam ruling](../../../../docs/ideas/behavior-selection/plan.md#providerconsumer-seam-check).

## Found

- New API compile-RED and old-picker/deal assertion failures become green; a virtual Unknown-as-eligible mutation is killed by row evidence and total1010versus10, not a build error — [implementation evidence](../../../../docs/ideas/behavior-selection/implementation.md).
- The existing two-role/one-row composer microbenchmark changes from13.075–13.611µs,4052–4053B/34allocs to14.813–15.134µs,5244–5247B/50allocs; these are copying/selection costs, not game-latency evidence — [measured cost](../../../../docs/ideas/behavior-selection/implementation.md#measured-cost).
- Full pre-change behavior-module race tests pass atba6d40f2; existing group tests pin separate parent/child dice and candidate evidence, which the adapter must retain — [slice verification](https://github.com/KirkDiggler/rpg-project/issues/918).

## Mistaken

- Initial task2 tests used an anonymous empty context key and an intentional nil-context call that lint rejected; the key is named and only the negative nil-context control has a documented local SA1012 annotation, with runtime/global lint unchanged — [implementation qualifications](../../../../docs/ideas/behavior-selection/implementation.md#qualifications-and-changes-to-the-plan).
