# Run — 2026-11-06 — canonical behavior consumer, in progress

Part of [NPC behavior S1 #866](https://github.com/KirkDiggler/rpg-project/issues/866), under [story #859](https://github.com/KirkDiggler/rpg-project/issues/859). One release unit: `rulebooks/dnd5e/encounter`, including dungeonspec.

## Shipped

- Implemented the first consumer checkpoint: selected-answer translation used by the existing driver, plus occurrence-owned heard kind through the real sound/queue/save/load/event path; canonical runtime cutover, source bindings, completion/pause and visible integration are not yet delivered — [consumer plan](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/plan-consumer.md#task-2-encounter-ownership-one-selection-and-actual-completion).

## Decided

- Translate the chosen answer through one existing mechanism rather than call a rolling driver after Composer.Decide, because a second selection changes the authored answer; the new public boundary carries only view/answer and returns the existing intent — [one-selection contract](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/selection-lane-contract.md).
- Carry heard kind on the queued occurrence itself and keep missing kind unknown, because a later voice must not rename an earlier clang; event delivery still exposes no maker — [Task 2](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/plan-consumer.md#task-2-encounter-ownership-one-selection-and-actual-completion).
- Pin the published provider commit for development, not a local replacement or a prerequisite merge; the consumer remains draft until the provider's actual tag and all integration proof exist — [provider #2292](https://github.com/KirkDiggler/rpg-toolkit/pull/2292).
- This checkpoint retains the existing runner while the replacement is incomplete, not as the final dual-runtime architecture or a compatibility adapter; the same draft must remove it before claiming canonical adoption — [R9 ruling](https://github.com/KirkDiggler/rpg-project/pull/877#issuecomment-6698028412).

## Found

- Real makeSound → EncounterData JSON → LoadEncounter → listener event lost both clang and voice kinds before the fix: expected each named kind, got nil. Both now survive in landing order; missing kind stays empty despite existing sound holdings — [required occurrence assertion](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/plan-consumer.md#task-2-encounter-ownership-one-selection-and-actual-completion).
- Real Composer.Decide on exact d200 face 101 selects the second authored goal; IntentForAnswer preserves that goal with one die call despite an unrelated table in the view. Focused new/translation/hearing controls pass in 0.076s; lint reports zero issues — [one-selection contract](https://github.com/KirkDiggler/rpg-project/blob/docs/consumer-patrol/ideas/behavior-bindings/selection-lane-contract.md).

- The complete owning-module `go test -race -count=1 ./...` passes: root 149.968s, castle 49.701s, dungeonspec 51.061s, keepgen 94.540s, replay 55.330s and all other packages; this verifies the preparatory checkpoint, not the still-unimplemented canonical journey — [consumer scope](https://github.com/KirkDiggler/rpg-project/issues/866).

## Mistaken

- The new reload test initially supplied a pointer where LoadEncounterInput.Data is a value; that compilation failure was corrected before the meaningful lost-kind assertion red and is not counted as regression evidence. The translator's missing-API compilation red is likewise distinct from assertion-level evidence — [owning data boundary](https://github.com/KirkDiggler/rpg-toolkit/blob/aca78cdf0714ab249ebd223923dfccf77cf2b3f9/rulebooks/dnd5e/encounter/data.go).
