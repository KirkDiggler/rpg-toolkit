# Run — 2026-11-05 — NPC behavior S2 authored finite sequences

processed: 2026-11-12T165555Z

Slice [rpg-project#871](https://github.com/KirkDiggler/rpg-project/issues/871), story [#859](https://github.com/KirkDiggler/rpg-project/issues/859). Base: toolkit main at [`d05c2fcd317e5bf979b62f4419a2b4595dd9697c`](https://github.com/KirkDiggler/rpg-toolkit/commit/d05c2fcd317e5bf979b62f4419a2b4595dd9697c).

## Shipped

- Implemented `mind/behavior.ExpandSequence`, authored JSON source fixtures, structural/ownership/parameter tests and joined component journeys; publication/review is separate from merge and live adoption — [slice brief](https://github.com/KirkDiggler/rpg-project/issues/871).

- Added the joined real-component Patrol traces (West 0→4→0, Court 10→12→14→10), exact d200 investigation branches, completion/hold/reload checks and the non-movement repeated-lines control — [toolkit#2284](https://github.com/KirkDiggler/rpg-toolkit/pull/2284).

- Added the source → binding → expanded-output walkthrough and producer godoc, including caller-owned actual completion, original-configuration reload and explicit studio/world limits — [toolkit#2284](https://github.com/KirkDiggler/rpg-toolkit/pull/2284).

## Decided

- The reusable producer utility lives beside the recipe/table composition it emits, not inside runtime execution; no existing composer/machine execution code, dependency or content field changes — [checked component plan](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md).
- The body uses canonical recipe/table JSON and four whole-value references, not string interpolation or arbitrary scripts; scalar checking reuses `CheckParameters`, while catalog/default/domain handling remains with existing producers — [component contract](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md#scope-and-concrete-producer-contract).
- Item-local state/table/bag names are qualified; generated completion beats include attachment identity and source maps preserve the authored item/name; anchor/wake remain instance-wide and no asynchronous job-token guarantee is invented — [component contract](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md#scope-and-concrete-producer-contract).

## Found

- Initial API tests failed to compile because the producer types/function were missing; after implementation, the owning-module race suite passes (root 1.153s, sound 1.016s, stage 1.016s; deed has no tests) and lint reports zero issues; these are component checks, not studio/live adoption — [Task 1 contract](https://github.com/KirkDiggler/rpg-project/blob/docs/behavior-bindings/ideas/behavior-bindings/plan-s2.md#task-1-production-finite-expansion-and-refusals).

- The joined checks pass with production expansion, real machine/composer/picker and exact dice; full module race passes (root 1.197s, sound 1.017s, stage 1.017s) and lint reports zero issues — [toolkit#2284](https://github.com/KirkDiggler/rpg-toolkit/pull/2284).

- Two intentional production mutants fail at assertions: wrong `$next` yields West 0 instead of 4 / Court 10 instead of 12, and a completion map pointing to `time` trips `ErrSelectionRequired`; both restored and the production SHA-256 matches the pre-mutation file — [regression-control receipt](https://github.com/KirkDiggler/rpg-toolkit/pull/2284).

## Mistaken

- Flattening `$next` to a string before local-name qualification allowed an authored name to capture it; a real assertion failed with expected `beta`, actual `captured`, and expansion now retains escaped source-reference locations so explicit next and literal local destinations remain distinct — [regression and fix](https://github.com/KirkDiggler/rpg-toolkit/pull/2284).
- The first joined-test run used positional dice-step literals and failed compilation because the existing helper also carries an error field; corrected to keyed literals, recorded as test setup error rather than behavioral red evidence — [joined test scope](https://github.com/KirkDiggler/rpg-toolkit/pull/2284).

- First fixture draft used uppercase position members; inspected canonical `spatial.Position` tags require lowercase `x`/`y`, corrected before the producer run rather than loosening strict decoding — [component test scope](https://github.com/KirkDiggler/rpg-project/issues/871).
