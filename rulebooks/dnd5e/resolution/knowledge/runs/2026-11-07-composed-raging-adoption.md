# 2026-11-07 — resolution consumes composed Raging

Work: [Composable conditions S12 #884](https://github.com/KirkDiggler/rpg-project/issues/884), approved [plan #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

## Shipped

- Resolution pins pushed D&D root 3e771a63 and constructs current Rage fixtures through the registry rather than the removed runtime type; no resolution rule or bus ownership changes — [pins](../../go.mod).
- Actual activation → boundary → next interaction verifies class payment, the first turn's dirty sheet and canonical progress/origin, then inactivity removal without fresh grace or a refunded charge. Full module race/vet/lint gates pass — [integration](../../raging_composition_test.go).

## Decided

- Preserve the existing real interaction/collector path and its gameplay assertions; only fixture construction and saved-shape reads adapt. No stored data is normalized or recreated implicitly — [dirty state test](../../dirty_test.go).

## Found

- Native QA exposed the inherited legacy damage-sustain subscription. TestNativeDamageSustainsRagingAcrossReload failed on root 3e771a63 after a real resisted NPC strike (HP 15 → 12), then passed on c00e0d76: positive applied damage persists activity, survives a new interaction/boundary once, then inactivity ends it; no legacy DamageReceived notification is emitted. Full module race/vet/lint pass — [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312).

- Updating the provider exposed compile errors at eight concrete legacy fixture sites, then one flat-state assertion failed. Current canonical fixture/reader changes restore those tests; the new three-interaction proof reaches the actual bus and dirty output, not a mock — [proof](../../raging_composition_test.go).

## Mistaken

- No new scope deviation observed; passing resolution does not complete session/API/web adoption or the operator's early native look — [remaining slice](https://github.com/KirkDiggler/rpg-project/issues/884).

— cross-team agent, on behalf of KirkDiggler
