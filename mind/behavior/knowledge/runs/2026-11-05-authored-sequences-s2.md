# Run — 2026-11-05 — NPC behavior S2 authored finite sequences

Slice [rpg-project#871](https://github.com/KirkDiggler/rpg-project/issues/871), story [#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit main at [`d05c2fcd317e5bf979b62f4419a2b4595dd9697c`](https://github.com/KirkDiggler/rpg-toolkit/commit/d05c2fcd317e5bf979b62f4419a2b4595dd9697c).

## Shipped

- Implemented the first working `mind/behavior.ExpandSequence` producer checkpoint, authored JSON source fixtures and structural/ownership/parameter tests; the slice's joined journey and final review remain in progress, and nothing is merged — [slice brief](https://github.com/KirkDiggler/rpg-project/issues/871).

## Decided

- The reusable producer utility lives beside the recipe/table composition it emits, not inside runtime execution; no existing composer/machine execution code, dependency or content field changes — [checked component plan](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md).
- The body uses canonical recipe/table JSON and four whole-value references, not string interpolation or arbitrary scripts; scalar checking reuses `CheckParameters`, while catalog/default/domain handling remains with existing producers — [component contract](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md#scope-and-concrete-producer-contract).
- Item-local state/table/bag names are qualified; generated completion beats include attachment identity and source maps preserve the authored item/name; anchor/wake remain instance-wide and no asynchronous job-token guarantee is invented — [component contract](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md#scope-and-concrete-producer-contract).

## Found

- Initial API tests failed to compile because the producer types/function were missing; after implementation, the owning-module race suite passes (root 1.153s, sound 1.016s, stage 1.016s; deed has no tests) and lint reports zero issues; these are component checks, not studio/live adoption — [Task 1 contract](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md#task-1-production-finite-expansion-and-refusals).

## Mistaken

- First fixture draft used uppercase position members; inspected canonical `spatial.Position` tags require lowercase `x`/`y`, corrected before the producer run rather than loosening strict decoding — [component test scope](https://github.com/KirkDiggler/rpg-project/issues/871).
