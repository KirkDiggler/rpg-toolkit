# Run — 2026-11-13 — Composable conditions S20 typed building blocks

Slice [rpg-project#956](https://github.com/KirkDiggler/rpg-project/issues/956), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [fefd3254](https://github.com/KirkDiggler/rpg-toolkit/commit/fefd3254).

## Shipped

- T2–T5 source checkpoints deliver explicit roll selection, typed construction/native Bless/Bane factories without namedRollParts or its panic, shared native Rage benefits, one code/data composition path and executable Go ring/order proof; consumer adoption, operator ergonomics acknowledgment and joined review/release remain separate gates — [slice](https://github.com/KirkDiggler/rpg-project/issues/956).

## Decided

- Kirk accepted independent instances, oldest applicable per group and signed addition, with R4 spelling allowed to evolve and a concentration-maintained raging ring as the usability measure — [approval](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6865141821).
- The root consumes neutral source185aae3c on its real pseudo-version; the private hardcoded group claimant is removed rather than retained beside the new selector — [provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2426).
- The native contribution grammar is exposed as ParseContributionDice for construction-time validation without fake source identity, instead of accepting the broader general dice parser or copying its narrower grammar — [parser](../../rolls/resolve.go).

## Found

- General dice.ParseNotation accepts mixed/modified pools, while the existing native contribution fold accepts unsigned homogeneous dice only; the shared parser closes that construction/execution mismatch — [parser tests](../../rolls/contribution_notation_test.go).
- The new AddToRoll, condition-selection and parser suites observed RED before their changes, then GREEN; full root race/vet/lint also pass at this checkpoint, including unchanged native Raging behavior — [selection proof](../../conditions/contribution_selection_test.go).

- Typed construction tests prove Go/data canonical equivalence, owned amount/guard/type values, nil/typed-nil refusal, payload-only output projection and the absence of implicit rest expiry; a failing named factory reaches preparation/create/load without initialization work — [typed tests](../../effects/typed_definitions_test.go).

- RagingBenefits supplies the same ordered mechanical outputs as the native recipe, preserves fixed/live-level amounts and owns each leaf's data; CompileComposition and the JSON library converge on one expanded validator and the existing binder. New producer/compiler suites and unchanged native regressions pass — [bundle proof](../../conditions/raging_benefits_test.go).

- The public Go ring example preserves an idle recipient across ten turn ends and reload, returns damage12/physical4, and removes the grant after a test-selected failure of its actual concentration follow-up; source budget stays spent — [executable example](../../conditions/typed_composition_example_test.go).
- Character and monster tests retain source order despite reversed runtime registration, then hand selection to the next instance without resetting its remaining duration; nonselected expiry and independent sibling resistance are also exercised — [character host proof](../../conditions/contribution_order_test.go).
- The countdown proof brought a typed Duration adapter to the existing kind, with missing-API RED then grace/countdown GREEN; character integration reuses the existing complete host instead of duplicating a simulation in another package — [duration test](../../effects/typed_duration_test.go).

## Mistaken

- The first immediate provider request resolved the repository root rather than its nested module; GitHub content and a direct module query established the correct subdir/hash, then the same pushed-hash request succeeded without a fabricated version or local override — [provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2426).
- The first focused build exposed two mechanical fixture migration misses (a test import alias and synthetic-provider Group field); both were corrected while retaining their behavioral assertions — [migration proof](../../conditions/baned_test.go).
- An existing named-immutability test still altered the retired params shape; it now changes the valid canonical Amount.Dice so refusal proves frozen named behavior rather than just schema rejection — [named round trip](../../conditions/named_linked_test.go).
