# 2026-11-06 — Rage and loot consume the extracted runtime

Work: [Rage/loot #2287](https://github.com/KirkDiggler/rpg-toolkit/pull/2287), [provider #2291](https://github.com/KirkDiggler/rpg-toolkit/pull/2291), [extraction #705](https://github.com/KirkDiggler/rpg-project/issues/705).

## Shipped

- Initial integration merged S1's provider re-pin, then migrated the Rage recipe/vocabulary to mechanics/effects and Compose(ComposeInput), resolving pushed provider e15c6e54 as v0.2.2-0.20261106234706-e15c6e54ef81, without replace/go.work — [go.mod](../../go.mod).
- Full module race tests, both runnable examples, vet and lint pass. go list dependency inventory contains no examples/effects package — [Rage tests](../../rage_item_test.go).

- Released adoption on 2026-11-07 merges S1's mechanics/effects v0.3.0 pin and main catch-up; full race tests, both examples, vet and lint pass again. S2 now targets main for real head CI — [dependency](../../go.mod).

## Decided

- Preserve the existing rulebook-shaped consumer, independent active effect lifetime, instance-local pools and host preparation/delivery boundary. This re-pin adds no production condition migration or persistence guarantee — [scope](../../RAGE.md).

## Found

- Stored-before-encounter loot restores identity and spent charges against the extracted provider; the damage/save/check chains and lifecycle tests still pass — [loot example](../../loot_example_test.go).

- Final adoption also uses S1's code-equivalent released resources v0.3.2 and D&D v0.213.5 pins; full race/example/vet/lint gates pass with no direct pseudo-versions remaining — [go.mod](../../go.mod).

## Mistaken

- Review caught stale future-tense provider-planning language in RAGE.md; it now links the completed generic extraction while keeping real storage and production integration explicitly unimplemented — [guide](../../RAGE.md).
- The initial stacked base could not supply main-target CI. S2 was retargeted before the parent merged/deleted; the released v0.3.0 pin and repeated local gates now permit the final head CI check — [PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2287).

— cross-team agent, on behalf of KirkDiggler
