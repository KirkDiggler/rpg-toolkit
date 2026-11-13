# Run — 2026-11-13 — Composable conditions S20 typed building blocks

Slice [rpg-project#956](https://github.com/KirkDiggler/rpg-project/issues/956), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [fefd3254](https://github.com/KirkDiggler/rpg-toolkit/commit/fefd3254).

## Shipped

- T2 source checkpoint adopts the pushed neutral selector, introduces current add_to_roll declarations with a single amount source and explicit polarity/selection, and joins actual-roll/information decisions; typed construction and the raging-ring proof remain pending — [slice](https://github.com/KirkDiggler/rpg-project/issues/956).

## Decided

- Kirk accepted independent instances, oldest applicable per group and signed addition, with R4 spelling allowed to evolve and a concentration-maintained raging ring as the usability measure — [approval](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6865141821).
- The root consumes neutral source185aae3c on its real pseudo-version; the private hardcoded group claimant is removed rather than retained beside the new selector — [provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2426).
- The native contribution grammar is exposed as ParseContributionDice for construction-time validation without fake source identity, instead of accepting the broader general dice parser or copying its narrower grammar — [parser](../../rolls/resolve.go).

## Found

- General dice.ParseNotation accepts mixed/modified pools, while the existing native contribution fold accepts unsigned homogeneous dice only; the shared parser closes that construction/execution mismatch — [parser tests](../../rolls/contribution_notation_test.go).
- The new AddToRoll, condition-selection and parser suites observed RED before their changes, then GREEN; full root race/vet/lint also pass at this checkpoint, including unchanged native Raging behavior — [selection proof](../../conditions/contribution_selection_test.go).

## Mistaken

- The first immediate provider request resolved the repository root rather than its nested module; GitHub content and a direct module query established the correct subdir/hash, then the same pushed-hash request succeeded without a fabricated version or local override — [provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2426).
- The first focused build exposed two mechanical fixture migration misses (a test import alias and synthetic-provider Group field); both were corrected while retaining their behavioral assertions — [migration proof](../../conditions/baned_test.go).
