# Run — 2026-11-08 — Composable conditions S14 rulebook adoption

Slice [rpg-project#904](https://github.com/KirkDiggler/rpg-project/issues/904), story [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [d4033752](https://github.com/KirkDiggler/rpg-toolkit/commit/d4033752); [approved plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md).

## Shipped

- Initial source checkpoint adopts provider cc0c8dee and the single typed amount contract for existing numeric meanings; application/codec/native work T6–T8 is not complete or released — [provider #2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327).

- T5 source supplies reusable concentration, duration, end-on, until-empty, resistance and query-only roll-modifier parts in the one kind table; native callers are not migrated by this vocabulary checkpoint — [T5 contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/plan.md#t5--rulebook-part-meanings-and-typed-contribution-adoption).

- T6 foundation adds the current linked codec, neutral registry entry, local assembly, detached information and per-part roll selection; transport/admission and named-condition retirement remain incomplete — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

- T7 source now prepares actual candidates, seals real delivery receipts, validates complete loaded graphs and transports ordered ends through real character keepers. Native caller wiring and old concentration retirement remain pending — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

## Decided

- Linked removals carry ApplicationID in addition to replacement address. The binding rejects stale/foreign facts before mutation, and both keeper implementations use one shared matcher so an old fact cannot strip a new invocation at the same address — [R4/R7](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/design.md#law).
- Pure Prepare validates linked part declarations without inventing a recipient. CreateRecipient validates adopted configs against an actual candidate and therefore belongs in native preflight before payment; Seal only accepts actually applied receipts and CheckComplete refuses provisional/orphaned state — [construction contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#construction-delivery-and-persistence).
- Keep one roll selector but expose an ordered contributor set per condition; each part retains its group, and metadata errors propagate. Plural information rows carry the same provider-owned group, so a mixed composition cannot flatten Bless/Bane or show a shadowed die as applying — [R5](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/design.md#law).
- Local payload attachment is not application admission: full graph validation belongs to the interaction binding, and an unbound application-ending part refuses transport rather than silently deleting a coordinator. This permits honest recipient-only projection without inventing a remote owner — [linked contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#construction-delivery-and-persistence).
- A reactive concentration output checks the provider's CanReact permission before requesting a save, separately from contributions still available during end reporting; this consumes provider eb5ffb63 rather than copying lifecycle state — [provider #2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327).
- RollQuery names the actual roll recipient, while a save-shaped outgoing action does not make its caster a saver; signed dice, selection group and source come from the same kind assessment used by information — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- Existing damage bonuses explicitly require scalar results; information uses nil for absent amounts. This is current numeric semantics, not a compatibility evaluator or a hidden die roll — [amount contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-composition/docs/ideas/composable-maintenance/README.md#describing-amounts-and-query-only-outputs).

## Found

- The first real replacement test accepted a stale old removal and deleted the new recipient; application-tagged binding/keeper checks turn that observed RED green. Mutants removing those checks fail both the real bound replacement and standalone-keeper regressions — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- Real keeper tests cover internal and externally initiated end order, last-member/duplicate notices, missing/foreign/competing ownership, pure preparation, actual-receipt sealing, zero delivery, self-recipient loss and independent payload survival; full root gates pass without treating these as browser or native-cast acceptance — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- The old registry Origin guard inferred support from the Raging-only composition field; explicit descriptor capability now admits linked creation without a ref-name switch. Existing registry coverage fixtures expose the new ref and retain their behavior checks — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- A plural-row selection mutant makes the second Bless source falsely apply and retain a benefit; the mixed-group execution/information assertion fails before exact restoration — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- Adopting the pushed provider first exposes four compile failures in the integer-only root evaluator/vocabulary; explicit scalar producers/consumers restore the existing conditions/effects tests without changing Raging gameplay — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

- New part APIs first fail by absence; resistance binding, nil-request attribution and payload inspection fail behaviorally before correction. Mutants dropping reaction permission, exact-owner qualification and subtraction fail the corresponding tests before exact restoration — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

## Mistaken

- An initial resistance fixture treated CreateFromRefOutput as its Condition; correcting the fixture precedes the meaningful unsupported-part RED. The first roll implementation also confused the single Answer.Roll with plural Effect rows; roll data now stays in the shared kind assessment without changing the information-row contract — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
