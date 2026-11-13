# 2026-11-07 — neutral support for the rulebook composition consumer

processed: 2026-11-12T165555Z

Work: [Composable conditions S9 #881](https://github.com/KirkDiggler/rpg-project/issues/881), under [#703](https://github.com/KirkDiggler/rpg-project/issues/703); approved [design/plan #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

## Shipped

- Development checkpoint adds full-Output kind validation, typed owned Snapshot, observational CurrentOutputs and safe params/host reads; no release or native adoption is claimed — [contracts](../../README.md#validation-and-observation-boundaries).
- Successful nonterminal transitions notify Changed after attachment; errors return to the publisher, while terminal End-before-detach and failed-attach behavior remain explicit — [transition](../../composed.go).

## Decided

- Keep rulebook origin/bindings and rich projections outside the provider; typed snapshots and bound-output observations give that owner its required seam without a JSON bridge or arbitrary domain payload — [approved contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/rulebook-effect-composition/docs/ideas/rulebook-effect-composition/README.md).
- Replace the narrow Check signature instead of adding a second competing validation callback; consumers adapt on real pushed pins — [vocabulary](../../vocabulary.go).

## Found

- The first nonterminal notification test failed on absent Changed; new observations/Check tests initially failed for the absent APIs/signature. Implemented contracts pass the real-bus suite — [tests](../../observations_test.go).
- Removing nonterminal Changed fails both the observed-state/damage test and callback-error propagation. Returning borrowed Params corrupts the next real attachment and is caught. Both mutants are restored — [mutation assertions](../../observations_test.go).

## Mistaken

- No new implementation mistake observed; the provider checkpoint does not substitute for rulebook restoration or native dirty-save evidence, which remain S10/S12 requirements — [plan](https://github.com/KirkDiggler/rpg-toolkit/blob/design/rulebook-effect-composition/docs/ideas/rulebook-effect-composition/plan.md).

— cross-team agent, on behalf of KirkDiggler
