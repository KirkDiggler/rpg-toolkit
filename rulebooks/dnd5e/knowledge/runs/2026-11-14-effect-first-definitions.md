# Run — 2026-11-14 — Composable conditions effect-first implementation

S20 [rpg-project#956](https://github.com/KirkDiggler/rpg-project/issues/956), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Replacement-plan baseline [13fdf1c9](https://github.com/KirkDiggler/rpg-toolkit/commit/13fdf1c94da0414c4a759dea1e1ed7f9c73af064).

## Shipped

- E1/E2 source checkpoints add canonical values, pure projection/intrinsic compilation, explicit placement, strict catalogue/reference resolution and whole-member derivation; native/consumer tasks remain pending and no release is claimed — [runtime PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2430).

## Decided

- Kirk accepted R9's ordered named resolved values and explicit placement, then R10's whole-member derivation and direction to proceed; the checked replacement plan is the implementation authority — [plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-building-blocks/docs/ideas/bless-bane-building-blocks/effect-first-plan.md).
- PartTemplate temporarily aliases the same canonical EffectSpec while the old catalogue adapter is migrated; there is no duplicated data or validator and no historical wire reader — [canonical effect type](../../effects/part_templates.go).

- DefineEntry is the measured typed-to-catalogue adapter: it projects through the same compiler into ordinary authored entries rather than adding a second interpretation path or a Go→JSON→Go round-trip — [catalogue](../../effects/catalogue.go).

## Found

- The compiler/placement suites observed missing-API RED then GREEN for standalone roots, explicit empty values, qualified order, duplicate/cyclic refusal, repeated subtree ownership, projection errors and placement coverage/scope checks; full root race/vet/lint pass — [compiler proof](../../effects/definition_compile_test.go).

- Catalogue/reference/derivation suites observed missing-API RED then GREEN; forward and same-entry acyclic references work, actual cycles/overlapping replacements/unused invalid entries refuse, and both reference-contract JSON sketches execute as fixtures with source values preserved — [reference proof](../../effects/reference_test.go).

## Mistaken

- A payload builder's registry read caused a Go initialization cycle; it now calls its kind-owned intrinsic validator directly while public declaration/placement validation retains table-owned scope checks — [validation owner](../../effects/effect_validation.go).
