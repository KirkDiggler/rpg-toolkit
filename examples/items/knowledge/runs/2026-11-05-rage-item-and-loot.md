# Run — 2026-11-05 — Keep equipment S2 Rage and encounter-independent loot

Slice [rpg-project#873](https://github.com/KirkDiggler/rpg-project/issues/873) of [story #850](https://github.com/KirkDiggler/rpg-project/issues/850), stacked on item example [toolkit#2273](https://github.com/KirkDiggler/rpg-toolkit/pull/2273) at [c6a84277](https://github.com/KirkDiggler/rpg-toolkit/commit/c6a842775972aebedf11b480dbd3ae39d022cb64).

## Shipped

- No merge, extraction or production release; the example adds a Rage consumer over real D&D chains and host-owned JSON retrieval/loot handoff, without modifying generic item/effects runtime code — [example](../../RAGE.md).

## Decided

- KirkDiggler selected items-first effects extraction on 2026-11-05, with condition migration later and built items usable outside encounters; the former combined promotion/condition-first brief is explicitly superseded pending re-planning — [provider direction](https://github.com/KirkDiggler/rpg-project/issues/703).
- Use representative one-charge/+2/long-rest fixture values and explicit caller association, because the objective is the composition seam rather than final content tuning — [plan](../../RAGE-PLAN.md).
- Keep persistence in the example host and keep stored definition/instance data separate, because encounter consumption must not imply creation or refill of existing loot — [retrieval tests](../../loot_example_test.go).
- Keep review proportionate to the example; focused tests and mutants supply the evidence, while extraction/production work retains its own gates — [slice contract](https://github.com/KirkDiggler/rpg-project/issues/873).

## Found

- The same item Use seam returns an unapplied effects.Composed; real machine transitions drive grace, sustain and endings while real D&D chains show the four Rage benefits with item-instance provenance — [Rage checks](../../rage_item_test.go).
- Item/grant association, item budget and applied Rage are independent: detaching the item does not secretly remove the result, ending the result does not refill the item, and recovery does not reactivate it — [lifetime checks](../../rage_item_test.go).
- Stored authored parameters are read rather than rebuilt from the fixture defaults; a spent item's identity and charge count survive another retrieval/handoff, and missing/null data refuses rather than creates — [artifact checks](../../loot_example_test.go).
- Apply can fail after Use spent the working copy's charge. The example host withholds the failed copy from persistence and reloads the original; this is not library rollback or a distributed commit guarantee — [delivery boundary](../../loot_example_test.go).
- Expiry-threshold and ignored-saved-state mutants fail their intended assertions; both are removed — [verification shape](../../RAGE.md#run).

## Mistaken

- Initial new tests did not compile before their consumer helpers existed; that is an interface checkpoint, not a provider behavioral defect. Lint identified an unused saved-state helper parameter and a missing contrasting actor test; the parameter was removed and other-actor activity is now exercised — [consumer tests](../../rage_item_test.go).
