# Run — 2026-11-14 — Composable conditions effect-first implementation

S20 [rpg-project#956](https://github.com/KirkDiggler/rpg-project/issues/956), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Replacement-plan baseline [13fdf1c9](https://github.com/KirkDiggler/rpg-toolkit/commit/13fdf1c94da0414c4a759dea1e1ed7f9c73af064).

## Shipped

- E1–E4 source checkpoints add canonical values/placement, strict catalogue/reference derivation, reusable activity/anchored-round maintenance and the complete typed Raging definition consumed by native Rage; linked adapter migration and downstream adoption remain pending and no release is claimed — [runtime PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2430).

## Decided

- Kirk accepted R9's ordered named resolved values and explicit placement, then R10's whole-member derivation and direction to proceed; the checked replacement plan is the implementation authority — [plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-building-blocks/docs/ideas/bless-bane-building-blocks/effect-first-plan.md).
- PartTemplate temporarily aliases the same canonical EffectSpec while the old catalogue adapter is migrated; there is no duplicated data or validator and no historical wire reader — [canonical effect type](../../effects/part_templates.go).

- DefineEntry is the measured typed-to-catalogue adapter: it projects through the same compiler into ordinary authored entries rather than adding a second interpretation path or a Go→JSON→Go round-trip — [catalogue](../../effects/catalogue.go).

- One-round anchored duration needs an explicit positive-round availability fact before any anchor exists; the added fact is a stateless read of the existing turn-end occurrence, not a new clock or actor lookup — [round policy](../../effects/round_duration.go).

- Native Rage uses the existing assembly with resolved local declarations and per-part snapshots; restore never reconstructs from a newer catalogue or accepts the old single bag. Shared kind inspection supplies progress through existing provider detail, not duplicate mutable reader state — [local host](../../conditions/composed.go).
- Required activity-grace and anchored-clock parts expose a derived first-turn-end witness; restore rejects contradictory grace without imposing a shared live bag or checking halfway through independent change notifications — [state validation](../../conditions/maintenance_info.go).
- Current rest fixtures serialize the real native creator and retain independently declared rest outcomes; the old monolithic dirty-count assertion now observes both independently changing parts through the same keeper contract — [rest proof](../../conditions/long_rest_registry_test.go).

- Application placement selects coordinator/recipient attachment independently from participant role and terminal scope; compilation uses the canonical compiler/placer, and the old binder temporarily delegates to the new single binding implementation while callers migrate — [application adapter](../../conditions/application_definition.go).

## Found

- The compiler/placement suites observed missing-API RED then GREEN for standalone roots, explicit empty values, qualified order, duplicate/cyclic refusal, repeated subtree ownership, projection errors and placement coverage/scope checks; full root race/vet/lint pass — [compiler proof](../../effects/definition_compile_test.go).

- Catalogue/reference/derivation suites observed missing-API RED then GREEN; forward and same-entry acyclic references work, actual cycles/overlapping replacements/unused invalid entries refuse, and both reference-contract JSON sketches execute as fixtures with source values preserved — [reference proof](../../effects/reference_test.go).

- New maintenance suites observe missing-API RED then GREEN for sparse/relative/missing/negative/reset clocks, first-end grace, positive damage/misses, local/application routing and inactivity precedence while contributions remain visible during ending; unchanged native suites also remain green — [assembly proof](../../effects/raging_maintenance_assembly_test.go).

- Complete-definition and local-host tests observed RED then GREEN: native factory consumes the actual discoverable benefit/upkeep fields; resolved saves are frozen; malformed coverage, terminal required upkeep and contradictory grace refuse; identity, payment and sparse-clock/terminal precedence proofs remain green with full root race/vet/lint — [native restore proof](../../conditions/composed_definition_test.go).
- Resolution and session tests directly reference the retired Runtime.State/Bag shape; separate current-fixture/provider-pin PRs are now required before API adoption rather than claiming API compilation verifies those modules — [resolution slice](https://github.com/KirkDiggler/rpg-project/issues/964), [session slice](https://github.com/KirkDiggler/rpg-project/issues/965).

- The first E5 checkpoint observes missing-API RED→GREEN for direct definitions, owned binding, grouped order, self-use addresses and custody refusals; recipient-local expiry plus explicit UntilEmpty cleans the existing lifetime after reload, and a real contradictory borrowed-upkeep restore fails before the shared validator is wired into the linked host — [application proof](../../conditions/application_definition_test.go).

## Mistaken

- The new local-expiry fixture initially used application-only Duration and assumed concentration implied UntilEmpty; it now selects supported RoundDuration and explicitly composes UntilEmpty, preserving both existing contracts rather than broadening production to fit a test — [explicit fixture](../../conditions/application_definition_test.go).
- The first round-state inspector checked the maximum only for live states; an added terminal-overflow assertion failed, then the shared bound check was corrected so malformed ended progress refuses too — [state proof](../../effects/round_duration_test.go).
- A payload builder's registry read caused a Go initialization cycle; it now calls its kind-owned intrinsic validator directly while public declaration/placement validation retains table-owned scope checks — [validation owner](../../effects/effect_validation.go).
