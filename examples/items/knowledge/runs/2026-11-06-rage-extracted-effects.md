# 2026-11-06 — Rage and loot consume the extracted runtime

Work: [Rage/loot #2287](https://github.com/KirkDiggler/rpg-toolkit/pull/2287), [provider #2291](https://github.com/KirkDiggler/rpg-toolkit/pull/2291), [extraction #705](https://github.com/KirkDiggler/rpg-project/issues/705).

## Shipped

- Merged S1's provider re-pin, then migrated the Rage recipe/vocabulary to mechanics/effects and Compose(ComposeInput); the module resolves pushed provider e15c6e54 as v0.2.2-0.20261106234706-e15c6e54ef81, without replace/go.work — [go.mod](../../go.mod).
- Full module race tests, both runnable examples, vet and lint pass. go list dependency inventory contains no examples/effects package — [Rage tests](../../rage_item_test.go).

## Decided

- Preserve the existing rulebook-shaped consumer, independent active effect lifetime, instance-local pools and host preparation/delivery boundary. This re-pin adds no production condition migration or persistence guarantee — [scope](../../RAGE.md).

## Found

- Stored-before-encounter loot restores identity and spent charges against the extracted provider; the damage/save/check chains and lifecycle tests still pass — [loot example](../../loot_example_test.go).

## Mistaken

- This stacked PR still targets the S1 branch, so no automatic main-target PR CI is claimed. Re-target before parent branch deletion, and re-pin/re-test against the actual provider tag after its authorized merge — [PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2287).

— cross-team agent, on behalf of KirkDiggler
